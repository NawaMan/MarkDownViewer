// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testServer(t *testing.T) (*viewServer, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub, err := fs.Sub(webRoot, "web")
	if err != nil {
		t.Fatal(err)
	}
	s := &viewServer{
		startRoot: root,
		root:      root,
		initialMd: "README.md",
		port:      8765,
		web:       sub,
		ghClient:  newGitHubClient(""),
	}
	return s, root
}

func TestAPITreeAndFile(t *testing.T) {
	s, _ := testServer(t)
	h := s.routes()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tree", nil)
	h.ServeHTTP(rr, req)
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
	req = httptest.NewRequest(http.MethodGet, "/api/file?path=README.md", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("file status %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "# Hello") {
		t.Fatalf("body %q", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/file?path=../etc/passwd", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("escape status %d", rr.Code)
	}
}

// A 1x1 PNG, enough to prove bytes come back untouched.
var onePixelPNG = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
}

func TestAPIAsset(t *testing.T) {
	s, root := testServer(t)
	h := s.routes()

	if err := os.MkdirAll(filepath.Join(root, "sub", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(root, "sub", "docs", "logo.png")
	if err := os.WriteFile(img, onePixelPNG, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", ".env"), []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/asset?path=sub/docs/logo.png", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("asset status %d: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type %q", got)
	}
	if !bytes.Equal(rr.Body.Bytes(), onePixelPNG) {
		t.Fatalf("asset body differs (%d bytes)", rr.Body.Len())
	}

	// Files outside the allowlist stay unreachable even though they sit
	// under the served root.
	for _, path := range []string{"sub/.env", "README.md"} {
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/asset?path="+path, nil)
		h.ServeHTTP(rr, req)
		if rr.Code != 403 {
			t.Fatalf("%s: status %d, want 403", path, rr.Code)
		}
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/asset?path=../../etc/hosts.png", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("escape status %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/asset?path=sub/docs/missing.png", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 404 {
		t.Fatalf("missing status %d", rr.Code)
	}
}

func TestAssetContentType(t *testing.T) {
	cases := map[string]string{
		"a/b/logo.PNG":   "image/png",
		"diagram.svg":    "image/svg+xml",
		"clip.mp4":       "video/mp4",
		"notes.md":       "",
		".env":           "",
		"secrets.json":   "",
		"Makefile":       "",
		"archive.tar.gz": "",
	}
	for name, want := range cases {
		if got := assetContentType(name); got != want {
			t.Errorf("assetContentType(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestAssetSVGIsSandboxed(t *testing.T) {
	s, root := testServer(t)
	h := s.routes()
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`
	if err := os.WriteFile(filepath.Join(root, "d.svg"), []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/asset?path=d.svg", nil))
	if rr.Code != 200 {
		t.Fatalf("svg status %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Security-Policy"); got != "sandbox" {
		t.Fatalf("CSP %q, want sandbox", got)
	}
}

func TestIndexAndMarked(t *testing.T) {
	s, _ := testServer(t)
	h := s.routes()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("index %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "viewmd") {
		t.Fatalf("index missing viewmd")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/vendor/marked.umd.js", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("marked %d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() < 1000 {
		t.Fatalf("marked too small")
	}
}

func TestConfig(t *testing.T) {
	s, _ := testServer(t)
	h := s.routes()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatal(rr.Code)
	}
	var cfg serverConfig
	if err := json.Unmarshal(rr.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.InitialMd != "README.md" || cfg.Port != 8765 {
		t.Fatalf("%+v", cfg)
	}
	// The server always reports a forward-slash path, even on Windows where
	// s.startRoot itself uses "\" — see the comment on serverConfig.
	if want := filepath.ToSlash(s.startRoot); cfg.StartFolder != want {
		t.Fatalf("StartFolder = %q, want %q", cfg.StartFolder, want)
	}
}

func postFolder(t *testing.T, h http.Handler, folder string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(setFolderRequest{Folder: folder})
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/folder", bytes.NewReader(body))
	h.ServeHTTP(rr, req)
	return rr
}

func TestSetFolderMovesWithinStart(t *testing.T) {
	s, root := testServer(t)
	h := s.routes()

	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "NOTES.md"), []byte("# Notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Absolute, inside startRoot: allowed.
	rr := postFolder(t, h, sub)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	newRoot, _ := s.state()
	if newRoot != sub {
		t.Fatalf("root = %q, want %q", newRoot, sub)
	}

	// Relative, resolved against startRoot (not the current root): "sub"
	// always means startRoot/sub, matching the mental model that the
	// starting directory — not wherever the server currently is — is what a
	// relative folder is anchored to.
	rr = postFolder(t, h, "sub")
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	newRoot, _ = s.state()
	if newRoot != sub {
		t.Fatalf("root after relative \"sub\" = %q, want %q", newRoot, sub)
	}

	// Absolute, back to startRoot itself: allowed.
	rr = postFolder(t, h, root)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	newRoot, _ = s.state()
	if newRoot != root {
		t.Fatalf("root after returning to startRoot = %q, want %q", newRoot, root)
	}
}

func TestSetFolderRejectsOutsideStart(t *testing.T) {
	s, root := testServer(t)
	h := s.routes()

	outside := t.TempDir()
	// A GitHub URL against a local startRoot must be rejected the same clean
	// way as any other out-of-bounds folder, not fall through to a raw
	// "no such file or directory" from treating it as a bogus relative path
	// (regression: it used to join onto root as a literal path segment).
	for _, bad := range []string{outside, "..", "../..", "https://github.com/NawaMan/CodingBooth"} {
		rr := postFolder(t, h, bad)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("folder=%q: status %d, want 400 (%s)", bad, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "starting directory") {
			t.Fatalf("folder=%q: body %q, want it to mention the starting directory", bad, rr.Body.String())
		}
	}

	// A rejected request must not have moved root.
	newRoot, _ := s.state()
	if newRoot != root {
		t.Fatalf("root = %q after rejected retargets, want unchanged %q", newRoot, root)
	}
}

func TestSetFolderRejectsFile(t *testing.T) {
	s, _ := testServer(t)
	h := s.routes()
	rr := postFolder(t, h, "README.md")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 for a file, not a directory", rr.Code)
	}
}

// unconfiguredTestServer mirrors testServer but with no startRoot: the
// state a --ask process is in before the browser's first pick (see
// viewServer.bootstrap in server.go and askInBrowser in main.go).
func unconfiguredTestServer(t *testing.T) *viewServer {
	t.Helper()
	sub, err := fs.Sub(webRoot, "web")
	if err != nil {
		t.Fatal(err)
	}
	return &viewServer{
		port:     8765,
		web:      sub,
		ghClient: newGitHubClient(""),
		askMode:  true,
	}
}

// Every handler that needs a root refuses with 503 until the server has
// been configured, rather than reading the process's own working directory.
func TestUnconfiguredServerRefusesUntilFirstPick(t *testing.T) {
	s := unconfiguredTestServer(t)
	h := s.routes()

	cfgRR := httptest.NewRecorder()
	h.ServeHTTP(cfgRR, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var cfg serverConfig
	if err := json.Unmarshal(cfgRR.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Configured {
		t.Fatalf("Configured = true before any pick, want false")
	}
	if !cfg.Ask {
		t.Fatalf("Ask = false, want true (askMode is set)")
	}

	for _, path := range []string{"/api/tree", "/api/file?path=README.md", "/api/asset?path=x.png"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("%s status = %d, want 503 before a folder is picked", path, rr.Code)
		}
	}
}

// The first /api/folder pick on an unconfigured server sets startRoot itself
// — unbounded, since there is nothing yet to bound it against — and every
// pick after that is an ordinary retarget bounded by whatever won.
func TestFirstFolderPickBootstrapsStartRoot(t *testing.T) {
	s := unconfiguredTestServer(t)
	h := s.routes()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()

	rr := postFolder(t, h, root)
	if rr.Code != http.StatusOK {
		t.Fatalf("first pick status %d: %s", rr.Code, rr.Body.String())
	}
	var cfg serverConfig
	if err := json.Unmarshal(rr.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.Configured || cfg.StartFolder != filepath.ToSlash(root) {
		t.Fatalf("cfg = %+v, want Configured=true and StartFolder=%s", cfg, root)
	}
	if got := s.getStartRoot(); got != root {
		t.Fatalf("startRoot = %q, want %q", got, root)
	}

	// Now bounded: a folder outside the just-picked startRoot is rejected,
	// same as for any other server.
	rr = postFolder(t, h, outside)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("post-bootstrap outside pick status %d, want 400", rr.Code)
	}
	if got := s.getStartRoot(); got != root {
		t.Fatalf("startRoot changed to %q after a rejected retarget, want unchanged %q", got, root)
	}
}

func TestResolveFreshRootLocal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "GUIDE.md"), []byte("# Guide\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gh := newGitHubClient("")

	rootAbs, initial, err := resolveFreshRoot(gh, root, "GUIDE.md")
	if err != nil {
		t.Fatalf("resolveFreshRoot: %v", err)
	}
	if rootAbs != root || initial != "GUIDE.md" {
		t.Fatalf("resolveFreshRoot = (%q, %q), want (%q, %q)", rootAbs, initial, root, "GUIDE.md")
	}

	if _, _, err := resolveFreshRoot(gh, filepath.Join(root, "GUIDE.md"), ""); err == nil {
		t.Fatal("resolveFreshRoot on a file, want an error")
	}

	if _, _, err := resolveFreshRoot(gh, filepath.Join(root, "does-not-exist"), ""); err == nil {
		t.Fatal("resolveFreshRoot on a missing directory, want an error")
	}
}
