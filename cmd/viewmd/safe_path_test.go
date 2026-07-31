// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUnderRoot_OK(t *testing.T) {
	root := t.TempDir()
	abs, rel, err := resolveUnderRoot(root, "docs/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if rel != "docs/a.md" {
		t.Fatalf("rel=%q", rel)
	}
	want := filepath.Join(root, "docs", "a.md")
	if abs != want {
		t.Fatalf("abs=%q want %q", abs, want)
	}
}

func TestResolveUnderRoot_RejectEscape(t *testing.T) {
	root := t.TempDir()
	if _, _, err := resolveUnderRoot(root, "../etc/passwd"); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := resolveUnderRoot(root, "foo/../../etc/passwd"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveUnderRoot_RejectAbs(t *testing.T) {
	root := t.TempDir()
	if _, _, err := resolveUnderRoot(root, root+"/x.md"); err == nil {
		t.Fatal("expected error for absolute")
	}
}

func TestBuildTree_FindsMarkdown(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "a.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// nested skip
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "no.md"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tree, err := buildTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 || tree.Files[0] != "README.md" {
		t.Fatalf("root files: %+v", tree.Files)
	}
	docs, ok := tree.Dirs["docs"]
	if !ok || len(docs.Files) != 1 || docs.Files[0] != "a.md" {
		t.Fatalf("docs: %+v", tree.Dirs)
	}
	paths, err := listMarkdownPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(paths, ",")
	if joined != "README.md,docs/a.md" {
		t.Fatalf("paths=%q", joined)
	}
}

func TestPeelExpose(t *testing.T) {
	set, host, rest, err := peelExpose([]string{"--folder", ".", "--expose", "--port", "9"})
	if err != nil || !set || host != "" {
		t.Fatalf("set=%v host=%q err=%v", set, host, err)
	}
	if len(rest) != 4 || rest[0] != "--folder" {
		t.Fatalf("rest=%v", rest)
	}

	set, host, rest, err = peelExpose([]string{"--expose", "18080", "--md", "R.md"})
	if err != nil || !set || host != "18080" {
		t.Fatalf("set=%v host=%q err=%v", set, host, err)
	}
	if len(rest) != 2 {
		t.Fatalf("rest=%v", rest)
	}

	set, host, _, err = peelExpose([]string{"--expose=+100"})
	if err != nil || !set || host != "+100" {
		t.Fatalf("set=%v host=%q err=%v", set, host, err)
	}

	_, _, _, err = peelExpose([]string{"--expose", "nope"})
	if err == nil {
		t.Fatal("expected invalid port error")
	}
}

func TestNormalizeInitialMd(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := normalizeInitialMd(root, "README.md")
	if err != nil || rel != "README.md" {
		t.Fatalf("rel=%q err=%v", rel, err)
	}
	rel, err = normalizeInitialMd(root, "./README.md")
	if err != nil || rel != "README.md" {
		t.Fatalf("rel=%q err=%v", rel, err)
	}
	_, err = normalizeInitialMd(root, "missing.md")
	if err == nil {
		t.Fatal("expected missing")
	}
}
