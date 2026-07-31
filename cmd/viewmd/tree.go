// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// treeNode is a nested directory of markdown file names (not full paths).
type treeNode struct {
	Files []string            `json:"files"`
	Dirs  map[string]treeNode `json:"dirs"`
}

var skipDirNames = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".booth":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"dist":         true,
	"build":        true,
}

func isMarkdown(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	default:
		return false
	}
}

func buildTree(root string) (treeNode, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return treeNode{}, err
	}
	rootAbs = filepath.Clean(rootAbs)

	// path (slash, relative) -> list of file basenames in that dir
	filesByDir := map[string][]string{}

	err = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip unreadable entries
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path == rootAbs {
				return nil
			}
			if skipDirNames[name] || strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !isMarkdown(name) {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "." {
			dir = ""
		}
		base := filepath.Base(rel)
		filesByDir[dir] = append(filesByDir[dir], base)
		return nil
	})
	if err != nil {
		return treeNode{}, err
	}

	for k := range filesByDir {
		sort.Strings(filesByDir[k])
	}

	// Ensure parent dirs exist as empty nodes when only nested files exist
	allDirs := map[string]struct{}{"": {}}
	for dir := range filesByDir {
		parts := strings.Split(dir, "/")
		cur := ""
		for _, p := range parts {
			if p == "" {
				continue
			}
			if cur == "" {
				cur = p
			} else {
				cur = cur + "/" + p
			}
			allDirs[cur] = struct{}{}
		}
		allDirs[dir] = struct{}{}
	}

	// Build bottom-up: start with leaf map of path -> node
	nodes := map[string]*treeNode{}
	for dir := range allDirs {
		n := &treeNode{Files: append([]string(nil), filesByDir[dir]...), Dirs: map[string]treeNode{}}
		if n.Files == nil {
			n.Files = []string{}
		}
		nodes[dir] = n
	}

	// Attach children to parents (deepest first)
	dirs := make([]string, 0, len(nodes))
	for d := range nodes {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		// deeper paths first
		return strings.Count(dirs[i], "/") > strings.Count(dirs[j], "/")
	})
	for _, d := range dirs {
		if d == "" {
			continue
		}
		parent := filepath.ToSlash(filepath.Dir(d))
		if parent == "." {
			parent = ""
		}
		base := filepath.Base(d)
		if p, ok := nodes[parent]; ok {
			// copy child value (not pointer) into map
			p.Dirs[base] = *nodes[d]
		}
	}

	rootNode := *nodes[""]
	if rootNode.Dirs == nil {
		rootNode.Dirs = map[string]treeNode{}
	}
	if rootNode.Files == nil {
		rootNode.Files = []string{}
	}
	return rootNode, nil
}

// listMarkdownPaths returns sorted relative slash paths of all markdown files.
func listMarkdownPaths(root string) ([]string, error) {
	t, err := buildTree(root)
	if err != nil {
		return nil, err
	}
	var out []string
	var walk func(prefix string, n treeNode)
	walk = func(prefix string, n treeNode) {
		for _, f := range n.Files {
			if prefix == "" {
				out = append(out, f)
			} else {
				out = append(out, prefix+"/"+f)
			}
		}
		names := make([]string, 0, len(n.Dirs))
		for k := range n.Dirs {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			p := k
			if prefix != "" {
				p = prefix + "/" + k
			}
			walk(p, n.Dirs[k])
		}
	}
	walk("", t)
	return out, nil
}
