// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// createLauncher writes a .lnk shortcut targeting the viewmd binary that
// created it, with --folder/--md baked into its Arguments. There is no pure
// Go way to write a .lnk without either a third-party COM binding or hand-
// rolling the MS-SHLLINK binary format, so this shells out to PowerShell's
// built-in WScript.Shell COM object — the same approach Windows' own scripting
// documentation recommends, and it needs nothing beyond what every Windows
// install already has.
func createLauncher(cfg launcherConfig) (string, error) {
	dir := cfg.OutputDir
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}
	lnkPath, err := filepath.Abs(filepath.Join(dir, cfg.Name+".lnk"))
	if err != nil {
		return "", fmt.Errorf("create-launcher: %w", err)
	}

	iconLocation, err := writeLauncherIcon(cfg, dir)
	if err != nil {
		return "", err
	}

	args := []string{"--folder", cfg.Folder}
	if cfg.InitialMd != "" {
		args = append(args, "--md", cfg.InitialMd)
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = quoteWindowsArg(a)
	}
	argString := strings.Join(quoted, " ")

	workDir := cfg.Folder
	if _, ok := parseGitHubURL(cfg.Folder); ok {
		workDir = filepath.Dir(cfg.ViewmdPath)
	}

	var script strings.Builder
	script.WriteString("$ws = New-Object -ComObject WScript.Shell\n")
	fmt.Fprintf(&script, "$sc = $ws.CreateShortcut(%s)\n", psSingleQuote(lnkPath))
	fmt.Fprintf(&script, "$sc.TargetPath = %s\n", psSingleQuote(cfg.ViewmdPath))
	fmt.Fprintf(&script, "$sc.Arguments = %s\n", psSingleQuote(argString))
	fmt.Fprintf(&script, "$sc.WorkingDirectory = %s\n", psSingleQuote(workDir))
	if iconLocation != "" {
		fmt.Fprintf(&script, "$sc.IconLocation = %s\n", psSingleQuote(iconLocation))
	}
	script.WriteString("$sc.Save()\n")

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("powershell: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return lnkPath, nil
}

// writeLauncherIcon writes a .ico file next to the shortcut and returns the
// IconLocation value for it ("" if cfg carries no icon).
func writeLauncherIcon(cfg launcherConfig, dir string) (string, error) {
	if len(cfg.IconData) == 0 {
		return "", nil
	}
	icoData := cfg.IconData
	switch cfg.IconExt {
	case ".png":
		converted, err := wrapPNGAsICO(cfg.IconData)
		if err != nil {
			return "", fmt.Errorf("icon: %w", err)
		}
		icoData = converted
	case ".ico":
		// used as-is
	default:
		return "", fmt.Errorf("icon: %s icons are not supported on Windows (use .png or .ico)", cfg.IconExt)
	}
	icoPath, err := filepath.Abs(filepath.Join(dir, cfg.Name+".ico"))
	if err != nil {
		return "", fmt.Errorf("icon: %w", err)
	}
	if err := os.WriteFile(icoPath, icoData, 0o644); err != nil {
		return "", fmt.Errorf("icon: %w", err)
	}
	return icoPath + ",0", nil
}

// quoteWindowsArg quotes a single argument the way CommandLineToArgvW expects
// to unquote it (the algorithm every Windows program built with the standard
// C runtime, and Go's own os/exec, relies on) — needed here because Arguments
// on a WshShortcut is one literal command-line string, not an argv slice.
func quoteWindowsArg(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n\v\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, r := range s {
		switch r {
		case '\\':
			slashes++
			b.WriteRune(r)
		case '"':
			for ; slashes > 0; slashes-- {
				b.WriteByte('\\')
			}
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			slashes = 0
			b.WriteRune(r)
		}
	}
	for ; slashes > 0; slashes-- {
		b.WriteByte('\\')
	}
	b.WriteByte('"')
	return b.String()
}

// psSingleQuote quotes s as a PowerShell single-quoted string literal, which
// (unlike a double-quoted one) never expands $variables or `escapes — the
// only special case is a literal single quote, doubled per PowerShell syntax.
func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
