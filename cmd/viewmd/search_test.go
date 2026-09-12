// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustMatcher(t *testing.T, query string, opts searchOptions) lineMatcher {
	t.Helper()
	m, err := newLineMatcher(query, opts)
	if err != nil {
		t.Fatalf("newLineMatcher(%q, %+v): %v", query, opts, err)
	}
	return m
}

func TestSnippetAround(t *testing.T) {
	short := "  # Hello World  "
	if got := snippetAround(short, mustMatcher(t, "hello", searchOptions{})); got != "# Hello World" {
		t.Fatalf("short snippet = %q", got)
	}

	long := strings.Repeat("a", 100) + "NEEDLE" + strings.Repeat("b", 100)
	got := snippetAround(long, mustMatcher(t, "needle", searchOptions{}))
	if !strings.Contains(got, "NEEDLE") {
		t.Fatalf("long snippet lost the match: %q", got)
	}
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
		t.Fatalf("long snippet not cropped on both ends: %q", got)
	}
	if len(got) >= len(long) {
		t.Fatalf("long snippet not shorter than the line: %d vs %d", len(got), len(long))
	}
}

func TestSearchFilesBasic(t *testing.T) {
	files := map[string]string{
		"a.md":     "# Title\nThis mentions apple and Banana.\nNothing else.\n",
		"b.md":     "no fruit here\n",
		"sub/c.md": "another apple reference\nAPPLE again\n",
	}
	paths := []string{"a.md", "b.md", "sub/c.md"}
	read := func(p string) ([]byte, error) { return []byte(files[p]), nil }

	resp, err := searchFiles(paths, "apple", searchOptions{}, 10, read)
	if err != nil {
		t.Fatalf("searchFiles: %v", err)
	}
	if resp.Truncated {
		t.Fatalf("unexpected truncation: %+v", resp)
	}
	if len(resp.Files) != 2 {
		t.Fatalf("expected 2 matching files, got %d: %+v", len(resp.Files), resp.Files)
	}
	byPath := map[string]searchFileResult{}
	for _, f := range resp.Files {
		byPath[f.Path] = f
	}
	if _, ok := byPath["a.md"]; !ok {
		t.Fatalf("a.md missing from results: %+v", resp.Files)
	}
	c, ok := byPath["sub/c.md"]
	if !ok || len(c.Matches) != 2 {
		t.Fatalf("sub/c.md should have 2 case-insensitive matches: %+v", c)
	}
	if c.Matches[0].Line != 1 || c.Matches[1].Line != 2 {
		t.Fatalf("unexpected line numbers: %+v", c.Matches)
	}
}

func TestSearchFilesCaseSensitive(t *testing.T) {
	files := map[string]string{
		"a.md": "apple\nAPPLE\n",
	}
	paths := []string{"a.md"}
	read := func(p string) ([]byte, error) { return []byte(files[p]), nil }

	resp, err := searchFiles(paths, "APPLE", searchOptions{CaseSensitive: true}, 10, read)
	if err != nil {
		t.Fatalf("searchFiles: %v", err)
	}
	if len(resp.Files) != 1 || len(resp.Files[0].Matches) != 1 || resp.Files[0].Matches[0].Line != 2 {
		t.Fatalf("expected exactly one case-sensitive match on line 2: %+v", resp.Files)
	}
}

func TestSearchFilesRegex(t *testing.T) {
	files := map[string]string{
		"a.md": "foo123\nfoobar\nbaz456\n",
	}
	paths := []string{"a.md"}
	read := func(p string) ([]byte, error) { return []byte(files[p]), nil }

	resp, err := searchFiles(paths, `foo\d+`, searchOptions{Regex: true}, 10, read)
	if err != nil {
		t.Fatalf("searchFiles: %v", err)
	}
	if len(resp.Files) != 1 || len(resp.Files[0].Matches) != 1 || resp.Files[0].Matches[0].Line != 1 {
		t.Fatalf("expected exactly one regex match on line 1: %+v", resp.Files)
	}

	if _, err := searchFiles(paths, `foo(`, searchOptions{Regex: true}, 10, read); err == nil {
		t.Fatal("expected an error for an invalid regex")
	}
}

func TestSearchFilesScanLimitTruncates(t *testing.T) {
	files := map[string]string{"a.md": "needle\n", "b.md": "needle\n"}
	paths := []string{"a.md", "b.md"}
	read := func(p string) ([]byte, error) { return []byte(files[p]), nil }

	resp, err := searchFiles(paths, "needle", searchOptions{}, 1, read)
	if err != nil {
		t.Fatalf("searchFiles: %v", err)
	}
	if !resp.Truncated {
		t.Fatal("expected Truncated when scanLimit stops before all paths are read")
	}
	if len(resp.Files) != 1 {
		t.Fatalf("expected exactly 1 scanned file's result, got %d", len(resp.Files))
	}
}

func TestSearchFilesUnreadableFileSkipped(t *testing.T) {
	paths := []string{"a.md", "b.md"}
	read := func(p string) ([]byte, error) {
		if p == "a.md" {
			return nil, os.ErrNotExist
		}
		return []byte("needle\n"), nil
	}
	resp, err := searchFiles(paths, "needle", searchOptions{}, 10, read)
	if err != nil {
		t.Fatalf("searchFiles: %v", err)
	}
	if len(resp.Files) != 1 || resp.Files[0].Path != "b.md" {
		t.Fatalf("expected only b.md to match, got %+v", resp.Files)
	}
}

func TestAPISearchLocal(t *testing.T) {
	s, root := testServer(t)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("# Guide\nSearch keyword lives here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := s.routes()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=keyword", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("search status %d: %s", rr.Code, rr.Body.String())
	}
	var resp searchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 1 || resp.Files[0].Path != "docs/guide.md" {
		t.Fatalf("unexpected result: %+v", resp)
	}

	// Empty query: no error, no results, no scan.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/search?q=", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("empty query status %d", rr.Code)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 0 {
		t.Fatalf("expected no results for empty query, got %+v", resp.Files)
	}

	// Unconfigured server refuses like every other data endpoint.
	unconf := &viewServer{web: s.web, ghClient: newGitHubClient(""), askDefault: root}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/search?q=keyword", nil)
	unconf.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured search status %d", rr.Code)
	}

	// case=1: "Keyword" (capitalized) should no longer match.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/search?q=Keyword&case=1", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("case-sensitive search status %d: %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 0 {
		t.Fatalf("expected no case-sensitive match for %q, got %+v", "Keyword", resp.Files)
	}

	// regex=1: a pattern that only a regex engine, not a substring search,
	// would match.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape(`key\w+`)+"&regex=1", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("regex search status %d: %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 1 || resp.Files[0].Path != "docs/guide.md" {
		t.Fatalf("unexpected regex result: %+v", resp)
	}

	// regex=1 with an invalid pattern is a client error, not a crash.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape("key(")+"&regex=1", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid regex status %d: %s", rr.Code, rr.Body.String())
	}
}
