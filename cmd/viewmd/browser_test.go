// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"os"
	"strings"
	"testing"
)

// The listen address and the address a browser can reach are not the same
// string: 0.0.0.0 is a valid thing to bind and not a valid thing to visit.
func TestBrowsableHost(t *testing.T) {
	for _, tc := range []struct{ bind, want string }{
		{"0.0.0.0", "127.0.0.1"},
		{"", "127.0.0.1"},
		{"::", "::1"},
		{"[::]", "::1"},
		{"0:0:0:0:0:0:0:0", "::1"},
		{"127.0.0.1", "127.0.0.1"},
		{"192.168.1.5", "192.168.1.5"},
		{"::1", "::1"},
		{"localhost", "localhost"},
		{"docs.internal", "docs.internal"},
	} {
		if got := browsableHost(tc.bind); got != tc.want {
			t.Errorf("browsableHost(%q) = %q, want %q", tc.bind, got, tc.want)
		}
	}
}

// IPv6 hosts have to come back bracketed, or the port reads as another group.
func TestBrowsableURL(t *testing.T) {
	for _, tc := range []struct {
		bind string
		port int
		want string
	}{
		{"0.0.0.0", 8765, "http://127.0.0.1:8765/"},
		{"127.0.0.1", 9000, "http://127.0.0.1:9000/"},
		{"::", 8765, "http://[::1]:8765/"},
		{"localhost", 80, "http://localhost:80/"},
	} {
		if got := browsableURL(tc.bind, tc.port); got != tc.want {
			t.Errorf("browsableURL(%q, %d) = %q, want %q", tc.bind, tc.port, got, tc.want)
		}
	}
}

// The banner shows the browsable URL, so a bind that answers on every
// interface — the default — has to say so somewhere or the loopback URL reads
// as the whole truth.
func TestBindNote(t *testing.T) {
	for _, tc := range []struct{ bind, want string }{
		{"0.0.0.0", "all interfaces"},
		{"", "all interfaces"},
		{"::", "all interfaces"},
		{"[::]", "all interfaces"},
		{"127.0.0.1", ""},
		{"192.168.1.5", ""},
		{"localhost", ""},
	} {
		if got := bindNote(tc.bind); got != tc.want {
			t.Errorf("bindNote(%q) = %q, want %q", tc.bind, got, tc.want)
		}
	}
}

// Every candidate must be runnable and must carry the URL, whatever the
// platform's launcher happens to be.
func TestBrowserCommandsCarryTheURL(t *testing.T) {
	const url = "http://127.0.0.1:8765/"
	cmds := browserCommands(url)
	if len(cmds) == 0 {
		t.Fatal("no launcher candidates for this platform")
	}
	for i, argv := range cmds {
		if len(argv) < 2 {
			t.Errorf("candidate %d = %v, want a program and at least the URL", i, argv)
			continue
		}
		if argv[len(argv)-1] != url {
			t.Errorf("candidate %d = %v, want the URL last", i, argv)
		}
	}
}

// A missing launcher is the common case on a headless box, so the loop has to
// walk past it rather than give up on the first entry.
func TestOpenBrowserFallsBackToNextLauncher(t *testing.T) {
	err := openBrowserWith([][]string{
		{"viewmd-no-such-launcher-a", "http://127.0.0.1:8765/"},
		{"viewmd-no-such-launcher-b", "http://127.0.0.1:8765/"},
		{os.Args[0], "-test.run=^TestHelperNoop$"},
	})
	if err != nil {
		t.Fatalf("openBrowserWith = %v, want the third candidate to be used", err)
	}
}

// With nothing to run, the caller needs an error it can put in a warning.
func TestOpenBrowserWithoutAnyLauncher(t *testing.T) {
	if err := openBrowserWith(nil); err == nil {
		t.Fatal("expected an error when there are no candidates")
	}

	err := openBrowserWith([][]string{{"viewmd-no-such-launcher", "http://127.0.0.1:8765/"}})
	if err == nil {
		t.Fatal("expected an error when no candidate can start")
	}
	if !strings.Contains(err.Error(), "viewmd-no-such-launcher") {
		t.Errorf("error = %v, want it to name the launcher that failed", err)
	}
}

// TestHelperNoop is not a real test: it is the stand-in browser launcher for
// TestOpenBrowserFallsBackToNextLauncher, which re-execs this binary.
func TestHelperNoop(t *testing.T) {}
