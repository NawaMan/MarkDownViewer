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
	mux.HandleFunc("/api/asset", s.handleAsset)
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

// handleAsset serves the images, media and fonts a Markdown page embeds or
// links to. The viewer resolves those paths against the directory of the file
// that mentions them before asking, so `path` is already root-relative here.
func (s *viewServer) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := r.URL.Query().Get("path")
	abs, _, err := resolveUnderRoot(s.root, rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctype := assetContentType(abs)
	if ctype == "" {
		http.Error(w, "not a file type viewmd serves", http.StatusForbidden)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if st.IsDir() {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// An SVG is a document, not just a picture: opened as a top-level page it
	// would run its own scripts on this origin. Inside <img> it is already
	// inert and the header is ignored, so this costs nothing where it matters.
	if strings.EqualFold(filepath.Ext(abs), ".svg") {
		w.Header().Set("Content-Security-Policy", "sandbox")
	}
	// ServeContent rather than io.Copy: it answers Range requests, which is
	// what lets a browser seek in an embedded video.
	http.ServeContent(w, r, filepath.Base(abs), st.ModTime(), f)
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
