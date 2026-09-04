// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"net"
	"strconv"
	"testing"
)

// pickFreePort exists specifically so a launcher never fights another viewmd
// instance (or another launcher) for the default 8765 — it has to return a
// port nothing is already listening on.
func TestPickFreePort(t *testing.T) {
	port, err := pickFreePort()
	if err != nil {
		t.Fatalf("pickFreePort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("pickFreePort = %d, want a valid TCP port", port)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("port %d returned by pickFreePort is not actually free: %v", port, err)
	}
	ln.Close()
}

func TestSanitizeFileName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Docs", "Docs"},
		{"My Docs", "My Docs"},
		{"a/b\\c:d*e?f", "a-b-c-d-e-f"},
		{"  spaced  ", "spaced"},
		{"...", ""},
		{"v1.2", "v1.2"},
	} {
		if got := sanitizeFileName(tc.in); got != tc.want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripKnownLauncherExt(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Docs", "Docs"},
		{"Docs.desktop", "Docs"},
		{"Docs.DESKTOP", "Docs"},
		{"Docs.app", "Docs"},
		{"Docs.lnk", "Docs"},
		{"Docs.md", "Docs.md"}, // not a launcher extension — left alone
	} {
		if got := stripKnownLauncherExt(tc.in); got != tc.want {
			t.Errorf("stripKnownLauncherExt(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDefaultLauncherName(t *testing.T) {
	for _, tc := range []struct{ rootAbs, want string }{
		{"/home/nawa/docs", "docs"},
		{"/", "viewmd"},
		{".", "viewmd"},
		{"https://github.com/OWNER/REPO/tree/main/docs", "REPO"},
	} {
		if got := defaultLauncherName(tc.rootAbs); got != tc.want {
			t.Errorf("defaultLauncherName(%q) = %q, want %q", tc.rootAbs, got, tc.want)
		}
	}
}

// --output is a path; a typo'd or copy-pasted launcher extension on it should
// not end up doubled (Docs.desktop.desktop).
func TestResolveLauncherOutput(t *testing.T) {
	tests := []struct {
		name          string
		output, root  string
		wantDir, want string
	}{
		{"empty output falls back to the folder's name", "", "/home/nawa/docs", ".", "docs"},
		{"explicit output, no extension", "/tmp/out/Docs", "/home/nawa/docs", "/tmp/out", "Docs"},
		{"explicit output, extension stripped", "/tmp/out/Docs.desktop", "/home/nawa/docs", "/tmp/out", "Docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, name := resolveLauncherOutput(tt.output, tt.root)
			if dir != tt.wantDir || name != tt.want {
				t.Errorf("resolveLauncherOutput(%q, %q) = (%q, %q), want (%q, %q)",
					tt.output, tt.root, dir, name, tt.wantDir, tt.want)
			}
		})
	}
}
