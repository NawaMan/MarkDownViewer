// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptForFolder(t *testing.T) {
	tests := []struct {
		name  string
		input string
		def   string
		want  string
	}{
		{"typed value overrides default", "docs\n", ".", "docs"},
		{"blank line keeps default", "\n", ".", "."},
		{"surrounding whitespace is trimmed", "  docs  \n", ".", "docs"},
		{"EOF with nothing typed keeps default", "", ".", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prompt strings.Builder
			got, err := promptForFolder(strings.NewReader(tt.input), &prompt, tt.def)
			if err != nil {
				t.Fatalf("promptForFolder: %v", err)
			}
			if got != tt.want {
				t.Errorf("promptForFolder(%q, def=%q) = %q, want %q", tt.input, tt.def, got, tt.want)
			}
			if !strings.Contains(prompt.String(), tt.def) {
				t.Errorf("prompt %q does not mention default %q", prompt.String(), tt.def)
			}
		})
	}
}

// captureStdout redirects os.Stdout for the duration of fn.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	out := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		out <- string(b)
	}()
	defer func() {
		os.Stdout = orig
		_ = r.Close()
	}()
	fn()
	_ = w.Close()
	return <-out
}

// The version subcommand and the --version flag must agree.
func TestVersionCommandAndFlag(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}, {"-version"}} {
		code := -1
		out := captureStdout(t, func() { code = run(args) })
		if code != 0 {
			t.Errorf("run(%v) = %d, want 0", args, code)
		}
		if got := strings.TrimSpace(out); got != version {
			t.Errorf("run(%v) printed %q, want %q", args, got, version)
		}
	}
}

// Trailing arguments after the subcommand are ignored rather than rejected.
func TestVersionCommandIgnoresTrailingArgs(t *testing.T) {
	code := -1
	out := captureStdout(t, func() { code = run([]string{"version", "--port", "9999"}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if got := strings.TrimSpace(out); got != version {
		t.Fatalf("printed %q, want %q", got, version)
	}
}

// "version" is only a command in the leading position; elsewhere it stays an
// ordinary (and therefore rejected) argument.
func TestVersionNotACommandWhenNotFirst(t *testing.T) {
	if code := run([]string{"--port", "9999", "version"}); code == 0 {
		t.Fatal("expected non-zero exit for a stray trailing argument")
	}
}

// A bareword that isn't a reserved subcommand (version/stop/status) is
// treated as a folder argument (`viewmd DIR`), so a typo like "srve" fails
// as a bad folder rather than as an unknown command.
func TestUnrecognizedBarewordTreatedAsFolder(t *testing.T) {
	if code := run([]string{"does-not-exist-xyz"}); code != 1 {
		t.Fatalf("exit = %d, want 1 for a nonexistent folder", code)
	}
}

// A folder argument and an explicit --folder flag conflict.
func TestPositionalFolderConflictsWithFlag(t *testing.T) {
	if code := run([]string{".", "--folder", "."}); code != 2 {
		t.Fatalf("exit = %d, want 2 when both a folder argument and --folder are given", code)
	}
}

// The stop/status commands and their flag aliases must behave identically.
func TestStopAndStatusCommands(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "viewmd.pid")

	// status: exit 1 when nothing is running, whichever spelling is used.
	for _, args := range [][]string{
		{"status", "--pidfile", stale},
		{"--status", "--pidfile", stale},
	} {
		code := -1
		out := captureStdout(t, func() { code = run(args) })
		if code != 1 {
			t.Errorf("run(%v) = %d, want 1", args, code)
		}
		if !strings.Contains(out, "not running") {
			t.Errorf("run(%v) printed %q, want it to mention \"not running\"", args, out)
		}
	}

	// stop: exit 0 and clear a stale pid file, whichever spelling is used.
	for _, args := range [][]string{
		{"stop", "--pidfile", stale},
		{"--stop", "--pidfile", stale},
	} {
		if err := writePidFile(stale, 0x7FFFFFF0); err != nil {
			t.Fatal(err)
		}
		code := -1
		out := captureStdout(t, func() { code = run(args) })
		if code != 0 {
			t.Errorf("run(%v) = %d, want 0", args, code)
		}
		if !strings.Contains(out, "not running") {
			t.Errorf("run(%v) printed %q, want it to mention \"not running\"", args, out)
		}
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Errorf("run(%v) left the stale pid file behind", args)
		}
	}
}

// --ask has no terminal to prompt on inside a daemon child, so the two
// cannot be combined.
func TestAskCannotCombineWithDaemon(t *testing.T) {
	if code := run([]string{"--ask", "--daemon"}); code != 2 {
		t.Fatalf("run(--ask --daemon) = %d, want 2", code)
	}
}

// The two actions cannot be combined, in any spelling.
func TestStopAndStatusMutuallyExclusive(t *testing.T) {
	for _, args := range [][]string{
		{"--stop", "--status"},
		{"stop", "--status"},
		{"status", "--stop"},
	} {
		if code := run(args); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}

// version.txt is the single source of truth for the version, so the daemon
// banner quoted in the README must not drift away from it.
func TestREADMEDaemonBannerMatchesVersionFile(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "version.txt"))
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# viewmd v" + strings.TrimSpace(string(raw)) + " running in background"
	if !strings.Contains(string(readme), want) {
		t.Errorf("README daemon example does not quote %q", want)
	}
}
