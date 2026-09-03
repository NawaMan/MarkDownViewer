// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseGitHubURL(t *testing.T) {
	cases := []struct {
		in   string
		want ghRoot
	}{
		{"https://github.com/NawaMan/CodingBooth", ghRoot{"NawaMan", "CodingBooth", "", ""}},
		{"github.com/NawaMan/CodingBooth", ghRoot{"NawaMan", "CodingBooth", "", ""}},
		{"https://github.com/NawaMan/CodingBooth/tree/main", ghRoot{"NawaMan", "CodingBooth", "main", ""}},
		{"https://github.com/NawaMan/CodingBooth/tree/main/docs", ghRoot{"NawaMan", "CodingBooth", "main", "docs"}},
		{"https://github.com/NawaMan/CodingBooth/tree/main/docs/", ghRoot{"NawaMan", "CodingBooth", "main", "docs"}},
		{"https://github.com/NawaMan/CodingBooth.git", ghRoot{"NawaMan", "CodingBooth", "", ""}},
		{"https://github.com/o/r/tree/main/a/b?plain=1#readme", ghRoot{"o", "r", "main", "a/b"}},
	}
	for _, c := range cases {
		got, ok := parseGitHubURL(c.in)
		if !ok {
			t.Errorf("parseGitHubURL(%q): not recognized", c.in)
			continue
		}
		if got != c.want {
			t.Errorf("parseGitHubURL(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParseGitHubURLRejectsNonGitHub(t *testing.T) {
	for _, in := range []string{"./docs", "/home/user/project", "C:\\docs", "gitlab.com/o/r", ""} {
		if _, ok := parseGitHubURL(in); ok {
			t.Errorf("parseGitHubURL(%q) unexpectedly matched", in)
		}
	}
}

func TestGithubCanonicalURLRoundTrip(t *testing.T) {
	u := githubCanonicalURL("NawaMan", "CodingBooth", "main", "docs/guides")
	want := "https://github.com/NawaMan/CodingBooth/tree/main/docs/guides"
	if u != want {
		t.Fatalf("got %q want %q", u, want)
	}
	gh, ok := parseGitHubURL(u)
	if !ok || gh.Owner != "NawaMan" || gh.Repo != "CodingBooth" || gh.Ref != "main" || gh.Path != "docs/guides" {
		t.Fatalf("round trip mismatch: %+v ok=%v", gh, ok)
	}
}

func TestResolveGitHubPath(t *testing.T) {
	cases := []struct {
		root, rel, want string
		wantErr         bool
	}{
		{"docs", "logo.png", "docs/logo.png", false},
		{"docs", "sub/logo.png", "docs/sub/logo.png", false},
		{"docs", "../secrets.env", "", true},
		{"docs", "../../secrets.env", "", true},
		// ".." that stays inside root is fine (a link from docs/guides/x.md
		// back up to a sibling under docs); one that climbs out of root
		// itself is not, even one level.
		{"docs", "guides/../other.md", "docs/other.md", false},
		{"docs/guides", "../logo.png", "", true},
		{"", "README.md", "README.md", false},
		{"", "../README.md", "", true},
		{"docs", "/etc/passwd", "", true},
	}
	for _, c := range cases {
		got, err := resolveGitHubPath(c.root, c.rel)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveGitHubPath(%q,%q) = %q, want error", c.root, c.rel, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveGitHubPath(%q,%q) unexpected error: %v", c.root, c.rel, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveGitHubPath(%q,%q) = %q, want %q", c.root, c.rel, got, c.want)
		}
	}
}

func TestBuildTreeFromPathsMatchesLocalShape(t *testing.T) {
	tree := buildTreeFromPaths([]string{"README.md", "docs/a.md", "docs/guides/b.md"})
	if len(tree.Files) != 1 || tree.Files[0] != "README.md" {
		t.Fatalf("root files: %+v", tree.Files)
	}
	docs, ok := tree.Dirs["docs"]
	if !ok || len(docs.Files) != 1 || docs.Files[0] != "a.md" {
		t.Fatalf("docs: %+v", tree.Dirs)
	}
	guides, ok := docs.Dirs["guides"]
	if !ok || len(guides.Files) != 1 || guides.Files[0] != "b.md" {
		t.Fatalf("guides: %+v", docs.Dirs)
	}
}

// ── mock GitHub API ─────────────────────────────────────────────

// A 1x1 PNG, enough to prove bytes come back untouched.
var ghOnePixelPNG = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
}

// newMockGitHubAPI serves just enough of the GitHub REST API for the
// githubClient to browse a small fixed fake repo: a README, a nested doc,
// an image asset, and a dotfile that must stay unreachable.
func newMockGitHubAPI(t *testing.T) *httptest.Server {
	t.Helper()
	blobs := map[string][]byte{
		"sha-readme": []byte("# Hello\n"),
		"sha-guide":  []byte("# Guide\n"),
		"sha-logo":   ghOnePixelPNG,
		"sha-env":    []byte("SECRET=1\n"),
	}
	entries := []ghTreeEntry{
		{Path: "README.md", Type: "blob", Sha: "sha-readme", Size: int64(len(blobs["sha-readme"]))},
		{Path: "docs", Type: "tree", Sha: "sha-docs-dir"},
		{Path: "docs/guide.md", Type: "blob", Sha: "sha-guide", Size: int64(len(blobs["sha-guide"]))},
		{Path: "docs/img", Type: "tree", Sha: "sha-img-dir"},
		{Path: "docs/img/logo.png", Type: "blob", Sha: "sha-logo", Size: int64(len(blobs["sha-logo"]))},
		{Path: ".env", Type: "blob", Sha: "sha-env", Size: int64(len(blobs["sha-env"]))},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRepoInfo{DefaultBranch: "main"})
	})
	mux.HandleFunc("/repos/o/r/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghTreeResponse{Tree: entries})
	})
	mux.HandleFunc("/repos/o/r/git/blobs/", func(w http.ResponseWriter, r *http.Request) {
		sha := strings.TrimPrefix(r.URL.Path, "/repos/o/r/git/blobs/")
		data, ok := blobs[sha]
		if !ok {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(ghBlobResponse{
			Content:  base64.StdEncoding.EncodeToString(data),
			Encoding: "base64",
		})
	})
	mux.HandleFunc("/rate-limited/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "9999999999")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	})
	return httptest.NewServer(mux)
}

func newTestGithubClient(t *testing.T) (*githubClient, *httptest.Server) {
	t.Helper()
	srv := newMockGitHubAPI(t)
	t.Cleanup(srv.Close)
	c := newGitHubClient("")
	c.apiBase = srv.URL
	return c, srv
}

func TestGithubClientTreeAndReadFile(t *testing.T) {
	c, _ := newTestGithubClient(t)

	tree, err := c.Tree("o", "r", "main", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 || tree.Files[0] != "README.md" {
		t.Fatalf("root files: %+v", tree.Files)
	}
	docs, ok := tree.Dirs["docs"]
	if !ok || len(docs.Files) != 1 || docs.Files[0] != "guide.md" {
		t.Fatalf("docs: %+v", tree.Dirs)
	}

	data, err := c.ReadFile("o", "r", "main", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Hello\n" {
		t.Fatalf("content = %q", data)
	}

	img, err := c.ReadFile("o", "r", "main", "docs/img/logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img, ghOnePixelPNG) {
		t.Fatalf("image bytes differ (%d bytes)", len(img))
	}

	if _, err := c.ReadFile("o", "r", "main", "missing.md"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestGithubClientTreeUnderSubpath(t *testing.T) {
	c, _ := newTestGithubClient(t)
	tree, err := c.Tree("o", "r", "main", "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 || tree.Files[0] != "guide.md" {
		t.Fatalf("files under docs: %+v", tree.Files)
	}
	// docs/img holds only a non-Markdown asset, so — matching buildTree's
	// local-folder behavior — it does not appear as a directory node here.
	if len(tree.Dirs) != 0 {
		t.Fatalf("dirs under docs: %+v", tree.Dirs)
	}
}

func TestGithubClientDirAndFileExists(t *testing.T) {
	c, _ := newTestGithubClient(t)
	if ok, err := c.DirExists("o", "r", "main", "docs"); err != nil || !ok {
		t.Fatalf("docs should exist: ok=%v err=%v", ok, err)
	}
	if ok, err := c.DirExists("o", "r", "main", "nope"); err != nil || ok {
		t.Fatalf("nope should not exist: ok=%v err=%v", ok, err)
	}
	if ok, err := c.FileExists("o", "r", "main", "README.md"); err != nil || !ok {
		t.Fatalf("README.md should exist: ok=%v err=%v", ok, err)
	}
}

func TestGithubClientDefaultBranch(t *testing.T) {
	c, _ := newTestGithubClient(t)
	ref, err := c.DefaultBranch("o", "r")
	if err != nil || ref != "main" {
		t.Fatalf("ref=%q err=%v", ref, err)
	}
}

func TestGithubClientRateLimit(t *testing.T) {
	c, srv := newTestGithubClient(t)
	c.apiBase = srv.URL + "/rate-limited"
	_, err := c.DefaultBranch("o", "r")
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	ge, ok := err.(*githubStatusError)
	if !ok || ge.status != http.StatusTooManyRequests {
		t.Fatalf("err = %v, want a 429 githubStatusError", err)
	}
	if !strings.Contains(ge.msg, "--github-token") {
		t.Fatalf("message should mention --github-token: %q", ge.msg)
	}
}

func TestGithubClientRateLimitAuthedOmitsTokenHint(t *testing.T) {
	c, srv := newTestGithubClient(t)
	c.token = "sekret"
	c.apiBase = srv.URL + "/rate-limited"
	_, err := c.DefaultBranch("o", "r")
	ge, ok := err.(*githubStatusError)
	if !ok {
		t.Fatalf("err = %v, want githubStatusError", err)
	}
	if strings.Contains(ge.msg, "--github-token") {
		t.Fatalf("authed rate limit message should not suggest a token: %q", ge.msg)
	}
}

// ── end to end through the HTTP handlers ────────────────────────

func testGithubServer(t *testing.T) *viewServer {
	t.Helper()
	c, _ := newTestGithubClient(t)
	sub, err := fs.Sub(webRoot, "web")
	if err != nil {
		t.Fatal(err)
	}
	root := githubCanonicalURL("o", "r", "main", "")
	return &viewServer{
		startRoot: root,
		root:      root,
		port:      8765,
		web:       sub,
		ghClient:  c,
	}
}

func TestGithubBackedAPITreeAndFile(t *testing.T) {
	s := testGithubServer(t)
	h := s.routes()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/tree", nil))
	if rr.Code != 200 {
		t.Fatalf("tree status %d: %s", rr.Code, rr.Body.String())
	}
	var tree treeNode
	if err := json.Unmarshal(rr.Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 || tree.Files[0] != "README.md" {
		t.Fatalf("%+v", tree)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/file?path="+url.QueryEscape("README.md"), nil))
	if rr.Code != 200 {
		t.Fatalf("file status %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "# Hello") {
		t.Fatalf("body %q", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/file?path="+url.QueryEscape("../secrets.env"), nil))
	if rr.Code != 400 {
		t.Fatalf("escape status %d", rr.Code)
	}
}

func TestGithubBackedAPIAsset(t *testing.T) {
	s := testGithubServer(t)
	h := s.routes()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/asset?path="+url.QueryEscape("docs/img/logo.png"), nil))
	if rr.Code != 200 {
		t.Fatalf("asset status %d: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type %q", got)
	}
	if !bytes.Equal(rr.Body.Bytes(), ghOnePixelPNG) {
		t.Fatalf("asset body differs (%d bytes)", rr.Body.Len())
	}

	// .env sits in the tree but is not on the asset allowlist.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/asset?path="+url.QueryEscape(".env"), nil))
	if rr.Code != 403 {
		t.Fatalf(".env status %d, want 403", rr.Code)
	}
}

func TestGithubBackedSetFolder(t *testing.T) {
	s := testGithubServer(t)
	h := s.routes()

	rr := postFolder(t, h, githubCanonicalURL("o", "r", "main", "docs"))
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	newRoot, _ := s.state()
	if newRoot != githubCanonicalURL("o", "r", "main", "docs") {
		t.Fatalf("root = %q", newRoot)
	}

	// A different repo/ref/owner is outside the starting repository.
	for _, bad := range []string{
		githubCanonicalURL("other", "r", "main", ""),
		githubCanonicalURL("o", "other", "main", ""),
		githubCanonicalURL("o", "r", "other-branch", ""),
		"/home/user/somewhere",
	} {
		rr := postFolder(t, h, bad)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("folder=%q: status %d, want 400", bad, rr.Code)
		}
	}
}
