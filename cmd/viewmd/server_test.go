// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
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
