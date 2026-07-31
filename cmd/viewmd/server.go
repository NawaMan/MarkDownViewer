// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type serverConfig struct {
	Folder    string `json:"folder"`
	InitialMd string `json:"initialMd,omitempty"`
	Port      int    `json:"port"`
}

type viewServer struct {
	root      string // absolute folder root
	folderArg string // as user passed it (for display)
	initialMd string // relative path or empty
	port      int
	web       fs.FS
}

func (s *viewServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/tree", s.handleTree)
	mux.HandleFunc("/api/file", s.handleFile)
	mux.Handle("/vendor/", http.FileServer(http.FS(s.web)))
	mux.HandleFunc("/", s.handleIndex)
	return mux
}

func (s *viewServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(s.web, "index.html")
	if err != nil {
		http.Error(w, "viewer UI missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *viewServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, serverConfig{
		Folder:    s.folderArg,
		InitialMd: s.initialMd,
		Port:      s.port,
	})
}

func (s *viewServer) handleTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tree, err := buildTree(s.root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, tree)
}

func (s *viewServer) handleFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := r.URL.Query().Get("path")
	abs, _, err := resolveUnderRoot(s.root, rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !isMarkdown(filepath.Base(abs)) {
		http.Error(w, "not a markdown file", http.StatusBadRequest)
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// normalizeInitialMd turns the --md argument into a slash-relative path under root,
// or returns "" if empty. Verifies the file exists when non-empty.
func normalizeInitialMd(root, md string) (string, error) {
	md = strings.TrimSpace(md)
	if md == "" {
		return "", nil
	}
	// Allow absolute paths only if they resolve under root.
	if filepath.IsAbs(md) {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		rootAbs = filepath.Clean(rootAbs)
		mdAbs := filepath.Clean(md)
		rel, err := filepath.Rel(rootAbs, mdAbs)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return "", fmt.Errorf("--md is outside --folder")
		}
		md = rel
	} else {
		md = strings.TrimPrefix(filepath.ToSlash(md), "./")
	}
	abs, clean, err := resolveUnderRoot(root, md)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("--md: %w", err)
	} else if st.IsDir() {
		return "", fmt.Errorf("--md is a directory")
	}
	return clean, nil
}
