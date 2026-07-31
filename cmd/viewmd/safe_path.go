// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// resolveUnderRoot joins rel to root and ensures the result stays inside root.
// rel is a slash-separated path from the client (may use OS separators too).
func resolveUnderRoot(root, rel string) (abs string, cleanRel string, err error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	rootAbs = filepath.Clean(rootAbs)

	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", "", fmt.Errorf("empty path")
	}
	// Reject absolute client paths and Windows drive forms.
	if filepath.IsAbs(rel) || (len(rel) > 1 && rel[1] == ':') {
		return "", "", fmt.Errorf("absolute paths are not allowed")
	}
	// Normalize to OS path, then clean (collapses ..).
	relOS := filepath.FromSlash(rel)
	joined := filepath.Clean(filepath.Join(rootAbs, relOS))

	sep := string(filepath.Separator)
	if joined != rootAbs && !strings.HasPrefix(joined, rootAbs+sep) {
		return "", "", fmt.Errorf("path escapes folder root")
	}

	cleanRel, err = filepath.Rel(rootAbs, joined)
	if err != nil {
		return "", "", err
	}
	cleanRel = filepath.ToSlash(cleanRel)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, "../") {
		return "", "", fmt.Errorf("path escapes folder root")
	}
	return joined, cleanRel, nil
}
