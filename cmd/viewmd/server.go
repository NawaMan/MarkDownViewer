// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type serverConfig struct {
	Folder      string `json:"folder"`
	InitialMd   string `json:"initialMd,omitempty"`
	Port        int    `json:"port"`
	StartFolder string `json:"startFolder"`
}

// setFolderRequest is the body of POST /api/folder: it retargets a running
// server at a different directory without restarting it.
type setFolderRequest struct {
	Folder string `json:"folder"`
	Md     string `json:"md,omitempty"`
}

type viewServer struct {
	port int
	web  fs.FS

	// ghClient serves every GitHub-backed root this server ever has, current
	// or past: its blob cache is content-addressed (keyed by git sha) so it
	// stays valid across a retarget, even one that moves to a different repo.
	// It exists (with whatever token --github-token/$GITHUB_TOKEN resolved
	// to at startup) whether or not the server currently has a GitHub root,
	// so a later retarget to one needs no separate wiring.
	ghClient *githubClient

	// startRoot is the directory (or GitHub URL) viewmd was launched with. It
	// never changes, and it bounds every later retarget: /api/folder may move
	// root anywhere under startRoot, never above or beside it.
	startRoot string

	// root and initialMd change when /api/folder retargets the server, so
	// every read and write goes through mu.
	mu        sync.RWMutex
	root      string // absolute folder root
	initialMd string // relative path or empty
}

// state returns a consistent snapshot of the mutable fields above.
func (s *viewServer) state() (root, initialMd string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root, s.initialMd
}

// setFolder retargets the server at a new root. rootAbs must already be a
// validated, absolute directory.
func (s *viewServer) setFolder(rootAbs, initialMd string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root, s.initialMd = rootAbs, initialMd
}

func (s *viewServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/tree", s.handleTree)
	mux.HandleFunc("/api/file", s.handleFile)
	mux.HandleFunc("/api/asset", s.handleAsset)
	mux.HandleFunc("/api/folder", s.handleSetFolder)
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
	root, initialMd := s.state()
	writeJSON(w, serverConfig{
		Folder:      filepath.ToSlash(root),
		InitialMd:   initialMd,
		Port:        s.port,
		StartFolder: filepath.ToSlash(s.startRoot),
	})
}

func (s *viewServer) handleTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	root, _ := s.state()
	if gh, ok := parseGitHubURL(root); ok {
		tree, err := s.ghClient.Tree(gh.Owner, gh.Repo, gh.Ref, gh.Path)
		if err != nil {
			writeGitHubError(w, err)
			return
		}
		writeJSON(w, tree)
		return
	}
	tree, err := buildTree(root)
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
	root, _ := s.state()
	rel := r.URL.Query().Get("path")
	if gh, ok := parseGitHubURL(root); ok {
		repoPath, err := resolveGitHubPath(gh.Path, rel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !isMarkdown(path.Base(repoPath)) {
			http.Error(w, "not a markdown file", http.StatusBadRequest)
			return
		}
		data, err := s.ghClient.ReadFile(gh.Owner, gh.Repo, gh.Ref, repoPath)
		if err != nil {
			writeGitHubError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
		return
	}
	abs, _, err := resolveUnderRoot(root, rel)
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
	root, _ := s.state()
	rel := r.URL.Query().Get("path")
	if gh, ok := parseGitHubURL(root); ok {
		repoPath, err := resolveGitHubPath(gh.Path, rel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctype := assetContentType(repoPath)
		if ctype == "" {
			http.Error(w, "not a file type viewmd serves", http.StatusForbidden)
			return
		}
		data, err := s.ghClient.ReadFile(gh.Owner, gh.Repo, gh.Ref, repoPath)
		if err != nil {
			writeGitHubError(w, err)
			return
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if strings.EqualFold(path.Ext(repoPath), ".svg") {
			w.Header().Set("Content-Security-Policy", "sandbox")
		}
		http.ServeContent(w, r, path.Base(repoPath), time.Time{}, bytes.NewReader(data))
		return
	}
	abs, _, err := resolveUnderRoot(root, rel)
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

// handleSetFolder retargets the running server at a different directory
// without a restart: `viewmd DIR` sends this when an instance is already
// serving the requested port, and the web UI's "Change folder" control uses
// it too. The new folder must resolve inside startRoot — the directory viewmd
// was launched with — so this cannot be used to climb out to the rest of the
// filesystem; it carries the same trust boundary as every other endpoint
// here otherwise, since there is no auth: anyone who can reach this server
// can already read its whole current tree under startRoot, and retargeting
// only changes which part of that subtree is current (see assetTypes in
// asset.go for the separate limit on which file types ever leave the
// process).
func (s *viewServer) handleSetFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req setFolderRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	rootAbs, err := s.resolveWithinStart(req.Folder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if gh, ok := parseGitHubURL(rootAbs); ok {
		exists, err := s.ghClient.DirExists(gh.Owner, gh.Repo, gh.Ref, gh.Path)
		if err != nil {
			writeGitHubError(w, err)
			return
		}
		if !exists {
			http.Error(w, "folder not found in repository", http.StatusBadRequest)
			return
		}
	} else {
		st, err := os.Stat(rootAbs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !st.IsDir() {
			http.Error(w, "folder is not a directory", http.StatusBadRequest)
			return
		}
	}
	// The old initialMd almost certainly does not exist in the new tree, so it
	// is only kept when the request names a file that does exist under the
	// new root.
	initial, err := s.normalizeInitialMd(rootAbs, req.Md)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.setFolder(rootAbs, initial)
	writeJSON(w, serverConfig{
		Folder:      filepath.ToSlash(rootAbs),
		InitialMd:   initial,
		Port:        s.port,
		StartFolder: filepath.ToSlash(s.startRoot),
	})
}

// resolveWithinStart resolves folder to a cleaned root identifier and rejects
// it unless that root is startRoot itself or a descendant of it — no "..",
// no absolute path, no symlink-free traversal trick gets a request outside
// what viewmd started with. A relative folder is resolved against startRoot
// itself — the directory (or GitHub folder) viewmd was launched with, not
// wherever the server currently happens to be rooted — matching the mental
// model that the starting point is what every "change dir" relies on. The
// web UI never sends a relative folder; it always computes the absolute path
// (or, for GitHub, the full canonical URL) of whichever tree entry was
// clicked and sends that instead.
//
// A GitHub startRoot only ever accepts a GitHub folder in return, in the
// same repo and at the same ref — retargeting cannot switch a running
// server from browsing a repo to browsing the local disk, or vice versa, or
// hop to a different repo/branch than the one it started on.
func (s *viewServer) resolveWithinStart(folder string) (string, error) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return "", fmt.Errorf("folder is required")
	}

	if startGH, ok := parseGitHubURL(s.startRoot); ok {
		gh, ok := parseGitHubURL(folder)
		if !ok || !strings.EqualFold(gh.Owner, startGH.Owner) || !strings.EqualFold(gh.Repo, startGH.Repo) || gh.Ref != startGH.Ref {
			return "", fmt.Errorf("folder must be within the starting repository %s", s.startRoot)
		}
		startPath := strings.Trim(startGH.Path, "/")
		if startPath != "" && gh.Path != startPath && !strings.HasPrefix(gh.Path, startPath+"/") {
			return "", fmt.Errorf("folder must be within the starting directory %s", s.startRoot)
		}
		return githubCanonicalURL(gh.Owner, gh.Repo, gh.Ref, gh.Path), nil
	}

	driveRelative := len(folder) > 1 && folder[1] == ':'
	var candidate string
	if filepath.IsAbs(folder) || driveRelative {
		candidate = filepath.Clean(folder)
	} else {
		candidate = filepath.Clean(filepath.Join(s.startRoot, folder))
	}
	sep := string(filepath.Separator)
	if candidate != s.startRoot && !strings.HasPrefix(candidate, s.startRoot+sep) {
		return "", fmt.Errorf("folder must be within the starting directory %s", s.startRoot)
	}
	return candidate, nil
}

// normalizeInitialMd dispatches to the local or GitHub form of --md / a
// retarget's Md field, based on which kind of root rootAbs names.
func (s *viewServer) normalizeInitialMd(rootAbs, md string) (string, error) {
	if gh, ok := parseGitHubURL(rootAbs); ok {
		return normalizeInitialMdGitHub(s.ghClient, gh, md)
	}
	return normalizeInitialMdLocal(rootAbs, md)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// normalizeInitialMdLocal turns the --md argument into a slash-relative path
// under root, or returns "" if empty. Verifies the file exists when non-empty.
func normalizeInitialMdLocal(root, md string) (string, error) {
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
