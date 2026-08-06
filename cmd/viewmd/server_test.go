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
		root:      root,
		folderArg: root,
		initialMd: "README.md",
		port:      8765,
		web:       sub,
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
}
