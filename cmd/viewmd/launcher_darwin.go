// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build darwin

package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// createLauncher writes a minimal .app bundle: a shell script under
// Contents/MacOS that execs the viewmd binary that created it with
// --folder/--md/--port baked in, plus an Info.plist and (when an icon was
// given) a Contents/Resources/icon.icns. Finder launches it like any other
// app — no Terminal window, no code signing needed for a bundle that never
// leaves the machine it was built on.
func createLauncher(cfg launcherConfig) (string, error) {
	dir := cfg.OutputDir
	if dir == "" {
		dir = "."
	}
	appDir := filepath.Join(dir, cfg.Name+".app")
	macosDir := filepath.Join(appDir, "Contents", "MacOS")
	resourcesDir := filepath.Join(appDir, "Contents", "Resources")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}
	if err := os.MkdirAll(resourcesDir, 0o755); err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}

	hasIcon, err := writeLauncherIcon(cfg, resourcesDir)
	if err != nil {
		return "", err
	}

	args := []string{
		shSingleQuote(cfg.ViewmdPath),
		"--folder", shSingleQuote(cfg.Folder),
		"--port", strconv.Itoa(cfg.Port),
	}
	if cfg.InitialMd != "" {
		args = append(args, "--md", shSingleQuote(cfg.InitialMd))
	}
	script := "#!/bin/sh\nexec " + strings.Join(args, " ") + "\n"
	execPath := filepath.Join(macosDir, cfg.Name)
	if err := os.WriteFile(execPath, []byte(script), 0o755); err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}

	plist := buildInfoPlist(cfg.Name, hasIcon)
	if err := os.WriteFile(filepath.Join(appDir, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}

	abs, err := filepath.Abs(appDir)
	if err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}
	return abs, nil
}

// writeLauncherIcon writes Resources/icon.icns when cfg carries an icon,
// reporting whether it did.
func writeLauncherIcon(cfg launcherConfig, resourcesDir string) (bool, error) {
	if len(cfg.IconData) == 0 {
		return false, nil
	}
	icnsData := cfg.IconData
	switch cfg.IconExt {
	case ".png":
		converted, err := wrapPNGAsICNS(cfg.IconData)
		if err != nil {
			return false, fmt.Errorf("icon: %w", err)
		}
		icnsData = converted
	case ".icns":
		// used as-is
	default:
		return false, fmt.Errorf("icon: %s icons are not supported on macOS (use .png or .icns)", cfg.IconExt)
	}
	if err := os.WriteFile(filepath.Join(resourcesDir, "icon.icns"), icnsData, 0o644); err != nil {
		return false, fmt.Errorf("icon: %w", err)
	}
	return true, nil
}

func buildInfoPlist(name string, hasIcon bool) string {
	iconKey := ""
	if hasIcon {
		iconKey = "\t<key>CFBundleIconFile</key>\n\t<string>icon.icns</string>\n"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>%s</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundleIdentifier</key>
	<string>io.viewmd.launcher.%s</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0</string>
%s</dict>
</plist>
`, xmlEscape(name), xmlEscape(name), sanitizeBundleID(name), iconKey)
}

// sanitizeBundleID narrows name down to what a reverse-DNS bundle identifier
// segment allows: letters, digits, dots and hyphens.
func sanitizeBundleID(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "app"
	}
	return b.String()
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// shSingleQuote quotes s as a POSIX sh single-quoted string literal — a
// literal single quote cannot appear inside a single-quoted string, so it is
// closed, an escaped quote spliced in, and a new single-quoted string reopened.
func shSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
