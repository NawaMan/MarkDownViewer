// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ghRoot identifies a directory inside a GitHub repository: the base folder
// viewmd is (or can be retargeted to be) rooted at. Path is repo-relative,
// slash-separated, with no leading or trailing slash ("" means the repo
// root).
type ghRoot struct {
	Owner, Repo, Ref, Path string
}

// ghURLRe accepts the URL shown in a browser's address bar for a repo, a
// branch, or a folder within a branch:
//
//	github.com/OWNER/REPO
//	https://github.com/OWNER/REPO
//	https://github.com/OWNER/REPO/tree/REF
//	https://github.com/OWNER/REPO/tree/REF/PATH/TO/DIR
//
// A ref containing "/" (some branch names do) is not supported — there is no
// way to tell where the ref ends and the path begins without an extra API
// call, and viewmd would rather stay predictable than guess.
var ghURLRe = regexp.MustCompile(`(?i)^(?:https?://)?github\.com/([^/\s]+)/([^/\s]+?)(?:\.git)?(?:/tree/([^/\s]+)(?:/(.*))?)?/?$`)

// parseGitHubURL reports whether raw names a GitHub repo (or a folder inside
// one), returning its parts. Ref is "" when raw did not name one — the
// canonical URL viewmd reports back always includes it, so ambiguity here
// only ever exists for user-supplied input, never for viewmd's own state.
func parseGitHubURL(raw string) (ghRoot, bool) {
	raw = strings.TrimSpace(raw)
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	m := ghURLRe.FindStringSubmatch(raw)
	if m == nil {
		return ghRoot{}, false
	}
	return ghRoot{Owner: m[1], Repo: m[2], Ref: m[3], Path: strings.Trim(m[4], "/")}, true
}

// githubCanonicalURL is the form viewmd reports back for a GitHub root (in
// /api/config, /api/tree, /api/folder) and the form the sidebar's "set base
// folder" / ".." controls build by plain string concatenation — see
// resolveGitHubPath and the client-side absJoin/parentOfFolder in
// web/index.html, which rely on this always including ref and never a
// trailing slash.
func githubCanonicalURL(owner, repo, ref, subpath string) string {
	u := fmt.Sprintf("https://github.com/%s/%s/tree/%s", owner, repo, ref)
	if subpath = strings.Trim(subpath, "/"); subpath != "" {
		u += "/" + subpath
	}
	return u
}

// resolveGitHubPath joins rel (a link/asset target relative to rootPath, as
// found in a rendered Markdown page) onto rootPath and ensures the result
// does not climb above it — the same containment rule resolveUnderRoot
// enforces for a local folder, expressed in slash-separated segments since a
// GitHub path is never a real filesystem path. Returns the resulting
// repo-relative path.
func resolveGitHubPath(rootPath, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	rootPath = strings.Trim(rootPath, "/")
	joined, ok := joinSlashPath(rootPath, rel)
	if !ok {
		return "", fmt.Errorf("path escapes folder root")
	}
	if rootPath != "" && joined != rootPath && !strings.HasPrefix(joined, rootPath+"/") {
		return "", fmt.Errorf("path escapes folder root")
	}
	return joined, nil
}

// joinSlashPath joins rel onto base and collapses "." / ".." segments,
// mirroring the joinPath helper in web/index.html. ok is false when rel
// climbs above base with more ".." segments than base has to give.
func joinSlashPath(base, rel string) (joined string, ok bool) {
	parts := append(strings.Split(base, "/"), strings.Split(rel, "/")...)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		switch p {
		case "", ".":
			continue
		case "..":
			if len(out) == 0 {
				return "", false
			}
			out = out[:len(out)-1]
		default:
			out = append(out, p)
		}
	}
	return strings.Join(out, "/"), true
}

// pathSkipped reports whether p sits inside a directory buildTree would
// prune for a local folder (skipDirNames, or any dot-prefixed directory).
func pathSkipped(p string) bool {
	dir := path.Dir(p)
	if dir == "." {
		return false
	}
	for _, seg := range strings.Split(dir, "/") {
		if skipDirNames[seg] || strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

// githubStatusError carries the HTTP status a GitHub API failure should
// surface as, so handlers can pass it straight to http.Error instead of
// guessing.
type githubStatusError struct {
	status int
	msg    string
}

func (e *githubStatusError) Error() string { return e.msg }

// writeGitHubError answers r with the status a GitHub-backed handler's error
// should carry: whatever githubClient decided (rate limit, not-found, ...),
// or 502 for anything else the API itself rejected.
func writeGitHubError(w http.ResponseWriter, err error) {
	if ge, ok := err.(*githubStatusError); ok {
		http.Error(w, ge.msg, ge.status)
		return
	}
	http.Error(w, err.Error(), http.StatusBadGateway)
}

// rateLimitError builds the message GitHub's own rate limit deserves: what
// happened, when it clears, and — only when the request went out
// unauthenticated, since a token would not have helped otherwise — how to
// raise the ceiling.
func rateLimitError(resetHeader string, authed bool) error {
	when := ""
	if ts, err := strconv.ParseInt(resetHeader, 10, 64); err == nil {
		when = " (resets at " + time.Unix(ts, 0).Local().Format("15:04:05 MST") + ")"
	}
	msg := "GitHub API rate limit exceeded" + when
	if !authed {
		msg += " — set --github-token or the GITHUB_TOKEN env var to raise the limit from 60 to 5,000 requests/hour"
	}
	return &githubStatusError{status: http.StatusTooManyRequests, msg: msg}
}

// ghTreeEntry is one row of the Git Trees API response: a blob (file) or a
// tree (directory), recursively, under the ref's root commit.
type ghTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" | "tree" | "commit"
	Sha  string `json:"sha"`
	Size int64  `json:"size"`
}

type ghTreeResponse struct {
	Tree      []ghTreeEntry `json:"tree"`
	Truncated bool          `json:"truncated"`
}

type ghRepoInfo struct {
	DefaultBranch string `json:"default_branch"`
}

type ghBlobResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

const (
	// treeCacheTTL bounds how stale a repo listing can be before the next
	// /api/tree request pays for a fresh one — short enough that a push to
	// the branch being browsed shows up without restarting viewmd, long
	// enough that clicking around the sidebar does not burn API calls.
	treeCacheTTL = 60 * time.Second
	// maxCachedBlobSize keeps a runaway video or PDF from growing the
	// in-memory blob cache without bound; anything larger is still served,
	// just re-fetched (and re-decoded) on every request.
	maxCachedBlobSize = 5 << 20
)

type ghTreeCacheEntry struct {
	at        time.Time
	entries   []ghTreeEntry
	truncated bool
}

// githubClient talks to the GitHub REST API. Blob content is cached forever
// by sha — a git blob's content never changes for a given sha, so this is
// always safe — while a repo's tree listing is cached briefly (treeCacheTTL)
// since it can change on every push.
type githubClient struct {
	http    *http.Client
	token   string
	apiBase string // overridden by tests; otherwise https://api.github.com

	mu        sync.Mutex
	treeCache map[string]ghTreeCacheEntry
	blobCache map[string][]byte
}

func newGitHubClient(token string) *githubClient {
	return &githubClient{
		http:      &http.Client{Timeout: 20 * time.Second},
		token:     token,
		apiBase:   "https://api.github.com",
		treeCache: map[string]ghTreeCacheEntry{},
		blobCache: map[string][]byte{},
	}
}

// get issues an authenticated (when a token is configured) GET and decodes
// a JSON response into out, translating GitHub's error conventions —
// rate-limited 403s, 404s — into githubStatusError so callers and HTTP
// handlers can act on them without re-parsing anything.
func (c *githubClient) get(apiURL string, out any) error {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("github: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return fmt.Errorf("github: %w", err)
	}
	if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) &&
		resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return rateLimitError(resp.Header.Get("X-RateLimit-Reset"), c.token != "")
	}
	if resp.StatusCode == http.StatusNotFound {
		return &githubStatusError{
			status: http.StatusNotFound,
			msg:    "GitHub: not found (repo, branch, or path does not exist, or — if private — needs --github-token)",
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &githubStatusError{
			status: http.StatusBadGateway,
			msg:    fmt.Sprintf("GitHub API error (%s): %s", resp.Status, strings.TrimSpace(string(body))),
		}
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("github: could not parse response: %w", err)
		}
	}
	return nil
}

// DefaultBranch resolves owner/repo's default branch, for when a GitHub URL
// names no ref (a bare "github.com/owner/repo").
func (c *githubClient) DefaultBranch(owner, repo string) (string, error) {
	var info ghRepoInfo
	apiURL := fmt.Sprintf("%s/repos/%s/%s", c.apiBase, owner, repo)
	if err := c.get(apiURL, &info); err != nil {
		return "", err
	}
	if info.DefaultBranch == "" {
		return "", fmt.Errorf("github: could not determine the default branch for %s/%s", owner, repo)
	}
	return info.DefaultBranch, nil
}

// fetchTree returns the full recursive listing for owner/repo@ref, from
// cache when it is fresh enough.
func (c *githubClient) fetchTree(owner, repo, ref string) ([]ghTreeEntry, bool, error) {
	key := owner + "/" + repo + "@" + ref
	c.mu.Lock()
	if e, ok := c.treeCache[key]; ok && time.Since(e.at) < treeCacheTTL {
		c.mu.Unlock()
		return e.entries, e.truncated, nil
	}
	c.mu.Unlock()

	apiURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", c.apiBase, owner, repo, ref)
	var tr ghTreeResponse
	if err := c.get(apiURL, &tr); err != nil {
		return nil, false, err
	}

	c.mu.Lock()
	c.treeCache[key] = ghTreeCacheEntry{at: time.Now(), entries: tr.Tree, truncated: tr.Truncated}
	c.mu.Unlock()
	return tr.Tree, tr.Truncated, nil
}

// Tree lists the Markdown files under subpath as a treeNode with paths
// relative to subpath, the same shape buildTree returns for a local folder.
// A repo too large for one recursive listing (GitHub's own "truncated" flag)
// is reported with whatever prefix of the tree the API did return, rather
// than failing outright.
func (c *githubClient) Tree(owner, repo, ref, subpath string) (treeNode, error) {
	entries, _, err := c.fetchTree(owner, repo, ref)
	if err != nil {
		return treeNode{}, err
	}
	subpath = strings.Trim(subpath, "/")
	var paths []string
	for _, e := range entries {
		if e.Type != "blob" || !isMarkdown(path.Base(e.Path)) {
			continue
		}
		if subpath != "" && e.Path != subpath && !strings.HasPrefix(e.Path, subpath+"/") {
			continue
		}
		if pathSkipped(e.Path) {
			continue
		}
		rel := e.Path
		if subpath != "" {
			rel = strings.TrimPrefix(rel, subpath+"/")
		}
		paths = append(paths, rel)
	}
	return buildTreeFromPaths(paths), nil
}

// DirExists reports whether subpath names a directory in owner/repo@ref (or
// is "", the repo root) — used to validate a folder retarget before
// committing to it.
func (c *githubClient) DirExists(owner, repo, ref, subpath string) (bool, error) {
	entries, _, err := c.fetchTree(owner, repo, ref)
	if err != nil {
		return false, err
	}
	if subpath = strings.Trim(subpath, "/"); subpath == "" {
		return true, nil
	}
	for _, e := range entries {
		if e.Type == "tree" && e.Path == subpath {
			return true, nil
		}
	}
	return false, nil
}

// FileExists reports whether repoPath names a file (blob) in owner/repo@ref.
func (c *githubClient) FileExists(owner, repo, ref, repoPath string) (bool, error) {
	entries, _, err := c.fetchTree(owner, repo, ref)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.Type == "blob" && e.Path == repoPath {
			return true, nil
		}
	}
	return false, nil
}

// ReadFile returns the decoded content of repoPath in owner/repo@ref. It
// looks the blob sha up in the (cached) tree listing rather than the
// Contents API so that files over the Contents API's 1MB inline limit still
// work, and so a hit is a single blob fetch instead of two round trips.
func (c *githubClient) ReadFile(owner, repo, ref, repoPath string) ([]byte, error) {
	entries, _, err := c.fetchTree(owner, repo, ref)
	if err != nil {
		return nil, err
	}
	var sha string
	var size int64
	found := false
	for _, e := range entries {
		if e.Type == "blob" && e.Path == repoPath {
			sha, size, found = e.Sha, e.Size, true
			break
		}
	}
	if !found {
		return nil, &githubStatusError{status: http.StatusNotFound, msg: "file not found: " + repoPath}
	}

	c.mu.Lock()
	if data, ok := c.blobCache[sha]; ok {
		c.mu.Unlock()
		return data, nil
	}
	c.mu.Unlock()

	apiURL := fmt.Sprintf("%s/repos/%s/%s/git/blobs/%s", c.apiBase, owner, repo, sha)
	var blob ghBlobResponse
	if err := c.get(apiURL, &blob); err != nil {
		return nil, err
	}
	if blob.Encoding != "base64" {
		return nil, fmt.Errorf("github: unexpected blob encoding %q", blob.Encoding)
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("github: could not decode file content: %w", err)
	}

	if size <= maxCachedBlobSize {
		c.mu.Lock()
		c.blobCache[sha] = data
		c.mu.Unlock()
	}
	return data, nil
}

// normalizeInitialMdGitHub is the GitHub counterpart of
// normalizeInitialMdLocal: cleans up the --md argument (or a retarget's Md)
// into a path relative to root, verifying it names a real file.
func normalizeInitialMdGitHub(c *githubClient, root ghRoot, md string) (string, error) {
	md = strings.TrimSpace(md)
	if md == "" {
		return "", nil
	}
	md = strings.TrimPrefix(md, "./")
	repoPath, err := resolveGitHubPath(root.Path, md)
	if err != nil {
		return "", fmt.Errorf("--md: %w", err)
	}
	exists, err := c.FileExists(root.Owner, root.Repo, root.Ref, repoPath)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("--md: %s not found", md)
	}
	cleanRel := repoPath
	if root.Path != "" {
		cleanRel = strings.TrimPrefix(strings.TrimPrefix(repoPath, root.Path), "/")
	}
	return cleanRel, nil
}
