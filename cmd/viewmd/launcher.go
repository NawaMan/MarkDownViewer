// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// launcherConfig is what a platform-specific createLauncher needs to lay down
// a double-clickable launcher. Folder and InitialMd are already resolved and
// validated by resolveFreshRoot, so every createLauncher can bake them in
// verbatim as --folder/--md.
type launcherConfig struct {
	ViewmdPath string // absolute path to the viewmd binary the launcher should run
	Folder     string // baked --folder value: an absolute local path or a canonical GitHub URL
	InitialMd  string // baked --md value, "" if none

	IconPath string // original --icon path, absolute; "" if no icon was given
	IconData []byte // --icon file contents; nil if no icon was given
	IconExt  string // lowercase extension of --icon, e.g. ".png"

	OutputDir string // directory the launcher is written into
	Name      string // sanitized base name (no extension) for the launcher file/bundle
}

// createLauncher is implemented per platform (launcher_windows.go,
// launcher_darwin.go, launcher_unix.go) and returns the path it created.
// var createLauncherFunc is not needed: each platform file provides the
// single createLauncher symbol directly, selected at compile time by build
// tags.

// runCreateLauncher implements `viewmd create-launcher`.
func runCreateLauncher(args []string) int {
	flags := flag.NewFlagSet("viewmd create-launcher", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	folder := flags.String("folder", ".", "directory (or GitHub URL) the launcher should open")
	md := flags.String("md", "", "Markdown file (relative to --folder) to open first")
	icon := flags.String("icon", "", "image for the launcher's icon (.png; .ico on Windows and\n"+
		"                     .icns on macOS are also accepted and used as-is)")
	output := flags.String("output", "", "where to write the launcher (default: the folder's own\n"+
		"                     name, in the current directory)")

	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, `viewmd create-launcher — create a double-clickable launcher for a folder
                        (EXPERIMENTAL — see below)

Usage:
  viewmd create-launcher [flags]

Flags:
  --folder DIR     Directory (or GitHub URL) the launcher opens (default ".")
  --md FILE        Markdown file (relative to --folder) to open first
  --icon PATH      Image for the launcher's icon (.png; .ico on Windows and
                     .icns on macOS are also accepted and used as-is)
  --output PATH    Where to write the launcher (default: the folder's own
                     name, in the current directory); the platform-specific
                     extension (.desktop / .app / .lnk) is added for you

Creates:
  Linux, BSD   a .desktop file
  macOS        a .app bundle
  Windows      a .lnk shortcut

Examples:
  viewmd create-launcher --folder ~/docs --md README.md --icon ~/docs/icon.png
  viewmd create-launcher --folder . --output ~/Desktop/Docs

The launcher always runs whichever viewmd binary created it, from wherever
that binary happens to live — move or remove it and the launcher stops
working, the same as any other shortcut to a program.

EXPERIMENTAL: whether double-clicking the result actually launches it
depends on the desktop environment/file manager's own trust and MIME
handling, which varies by distro and version and has not been verified
everywhere. Report what you saw (which platform/DE, what happened) so this
can be hardened.
`)
	}

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments: %v\n", flags.Args())
		return 2
	}

	viewmdPath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: could not locate the viewmd binary:", err)
		return 1
	}
	if resolved, err := filepath.EvalSymlinks(viewmdPath); err == nil {
		viewmdPath = resolved
	}

	ghClient := newGitHubClient(resolveGitHubToken(""))
	rootAbs, initial, err := resolveFreshRoot(ghClient, *folder, *md)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}

	var iconData []byte
	var iconExt, iconAbsPath string
	if *icon != "" {
		data, err := os.ReadFile(*icon)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: icon:", err)
			return 1
		}
		iconData = data
		iconExt = strings.ToLower(filepath.Ext(*icon))
		if abs, err := filepath.Abs(*icon); err == nil {
			iconAbsPath = abs
		} else {
			iconAbsPath = *icon
		}
	}

	outDir, name := resolveLauncherOutput(*output, rootAbs)

	cfg := launcherConfig{
		ViewmdPath: viewmdPath,
		Folder:     rootAbs,
		InitialMd:  initial,
		IconPath:   iconAbsPath,
		IconData:   iconData,
		IconExt:    iconExt,
		OutputDir:  outDir,
		Name:       name,
	}

	created, err := createLauncher(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	fmt.Printf("viewmd: created launcher %s\n", created)
	fmt.Printf("  target: %s\n", rootAbs)
	if initial != "" {
		fmt.Printf("  initial file: %s\n", initial)
	}
	fmt.Fprintln(os.Stderr, "  (EXPERIMENTAL: double-click launch behavior depends on your desktop")
	fmt.Fprintln(os.Stderr, "   environment/file manager and has not been verified everywhere — if it")
	fmt.Fprintln(os.Stderr, "   doesn't run, that's a known gap, not something you're doing wrong.)")
	return 0
}

// resolveLauncherOutput splits --output into a directory and a sanitized base
// name, falling back to the target's own name in the current directory when
// --output is empty. A platform-specific launcher extension (.desktop/.app/
// .lnk) accidentally typed into --output is stripped, since each createLauncher
// appends its own.
func resolveLauncherOutput(output, rootAbs string) (dir, name string) {
	if output != "" {
		dir = filepath.Dir(output)
		name = filepath.Base(output)
	} else {
		dir = "."
		name = defaultLauncherName(rootAbs)
	}
	name = sanitizeFileName(stripKnownLauncherExt(name))
	if name == "" {
		name = "viewmd"
	}
	return dir, name
}

// defaultLauncherName picks a launcher name from the resolved target: the
// repo name for a GitHub root, otherwise the local directory's own name.
func defaultLauncherName(rootAbs string) string {
	if gh, ok := parseGitHubURL(rootAbs); ok && gh.Repo != "" {
		return gh.Repo
	}
	base := filepath.Base(rootAbs)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "viewmd"
	}
	return base
}

func stripKnownLauncherExt(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range []string{".desktop", ".app", ".lnk"} {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

// sanitizeFileName keeps a launcher name usable as a filename (or, on macOS,
// as an .app bundle directory name) on every supported platform.
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == ' ', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), " .-")
}
