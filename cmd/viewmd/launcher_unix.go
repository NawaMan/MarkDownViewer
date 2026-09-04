// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build !windows && !darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// createLauncher writes a freedesktop .desktop file that runs the viewmd
// binary that created it with --folder/--md baked in. Icon references the
// given icon file by its own absolute path — the desktop entry spec's Icon
// key accepts one directly, and most desktop environments load PNG or SVG
// icons from an arbitrary path with no conversion needed (.ico/.icns are
// passed through the same way, but support for those varies by icon theme
// engine).
func createLauncher(cfg launcherConfig) (string, error) {
	dir := cfg.OutputDir
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}
	path, err := filepath.Abs(filepath.Join(dir, cfg.Name+".desktop"))
	if err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}

	execArgs := []string{quoteDesktopExecArg(cfg.ViewmdPath), "--folder", quoteDesktopExecArg(cfg.Folder)}
	if cfg.InitialMd != "" {
		execArgs = append(execArgs, "--md", quoteDesktopExecArg(cfg.InitialMd))
	}

	var iconLine string
	if cfg.IconPath != "" {
		iconLine = fmt.Sprintf("Icon=%s\n", cfg.IconPath)
	}

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
%sTerminal=false
Categories=Utility;
`, cfg.Name, strings.Join(execArgs, " "), iconLine)

	// The executable bit is what most desktop environments require before
	// they will run a .desktop file on double-click rather than just show it
	// as text.
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}
	markDesktopFileTrusted(path)
	return path, nil
}

// markDesktopFileTrusted best-effort marks path as a trusted launcher via
// GNOME/Nautilus's own metadata channel. Since GNOME 3.36, Nautilus treats a
// freshly written .desktop file as untrusted and opens it as plain text on
// double-click instead of running it — the executable bit alone is no longer
// enough — until this same "Allow Launching" attribute is set, normally done
// by hand from the file's right-click menu. Every non-GNOME desktop, and a
// gio build without the gvfs metadata backend, simply rejects the attribute;
// that failure is silently ignored, since not being able to set a
// GNOME-specific flag is not a failure worth reporting for a feature meant
// to work the same way on every desktop environment. PATH is tried first, as
// is normal, then two common absolute paths in case something earlier on
// PATH shadows the system gio with a build that lacks gvfs support.
func markDesktopFileTrusted(path string) {
	for _, bin := range []string{"gio", "/usr/bin/gio", "/bin/gio"} {
		if exec.Command(bin, "set", path, "metadata::trusted", "yes").Run() == nil {
			return
		}
	}
}

// quoteDesktopExecArg quotes s per the Desktop Entry Specification's Exec
// key grammar when it contains characters outside the set that never needs
// quoting there.
func quoteDesktopExecArg(s string) string {
	safe := true
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			strings.ContainsRune("/._-+:@", r):
		default:
			safe = false
		}
		if !safe {
			break
		}
	}
	if safe {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '`', '$', '\\':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
