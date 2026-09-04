// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build !windows && !darwin

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateLauncherWritesDesktopFile(t *testing.T) {
	dir := t.TempDir()
	cfg := launcherConfig{
		ViewmdPath: "/opt/viewmd/viewmd",
		Folder:     "/home/nawa/docs",
		InitialMd:  "README.md",
		IconPath:   "/home/nawa/docs/icon.png",
		OutputDir:  dir,
		Name:       "Docs Launcher",
	}
	path, err := createLauncher(cfg)
	if err != nil {
		t.Fatalf("createLauncher: %v", err)
	}
	wantPath := filepath.Join(dir, "Docs Launcher.desktop")
	if path != wantPath {
		t.Errorf("createLauncher path = %q, want %q", path, wantPath)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("launcher is not executable; most desktop environments refuse to run it")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(data)
	for _, want := range []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Docs Launcher",
		"Exec=/opt/viewmd/viewmd --folder /home/nawa/docs --md README.md",
		"Icon=/home/nawa/docs/icon.png",
		"Terminal=false",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("desktop file does not contain %q:\n%s", want, content)
		}
	}
}

func TestCreateLauncherWithoutIconOmitsIconLine(t *testing.T) {
	cfg := launcherConfig{
		ViewmdPath: "/opt/viewmd/viewmd",
		Folder:     "/home/nawa/docs",
		OutputDir:  t.TempDir(),
		Name:       "Docs",
	}
	path, err := createLauncher(cfg)
	if err != nil {
		t.Fatalf("createLauncher: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(data), "Icon=") {
		t.Errorf("desktop file has an Icon= line with no --icon given:\n%s", data)
	}
}

func TestQuoteDesktopExecArg(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"/home/nawa/docs", "/home/nawa/docs"},
		{"/home/nawa/my docs", `"/home/nawa/my docs"`},
		{`/home/nawa/weird"name`, `"/home/nawa/weird\"name"`},
	} {
		if got := quoteDesktopExecArg(tc.in); got != tc.want {
			t.Errorf("quoteDesktopExecArg(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
