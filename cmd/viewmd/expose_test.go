// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubExposeOnPath installs a booth--expose script running body and puts it
// first on PATH for the duration of the test.
func stubExposeOnPath(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub tunnel is a shell script")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(filepath.Join(dir, "booth--expose"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestStartBoothExposeNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	e, err := startBoothExpose(8765, "")
	if err == nil {
		e.stop()
		t.Fatal("expected an error when booth--expose is not on PATH")
	}
	if e != nil {
		t.Fatal("expected a nil exposeProc alongside the error")
	}
}

// A tunnel that never exits must be terminated by stop.
func TestExposeStopTerminatesTunnel(t *testing.T) {
	dir := stubExposeOnPath(t, "while true; do sleep 1; done\n")
	_ = dir

	e, err := startBoothExpose(8765, "")
	if err != nil {
		t.Fatal(err)
	}
	pid := e.cmd.Process.Pid
	if !processAlive(pid) {
		t.Fatal("tunnel should be running")
	}

	e.stop()
	if processAlive(pid) {
		t.Fatalf("tunnel (pid %d) still alive after stop", pid)
	}
	// stop must be idempotent.
	e.stop()
}

// stop on a tunnel that already exited on its own returns promptly.
func TestExposeStopAfterNaturalExit(t *testing.T) {
	stubExposeOnPath(t, "exit 0\n")

	e, err := startBoothExpose(8765, "")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-e.done:
	case <-time.After(5 * time.Second):
		t.Fatal("tunnel did not exit on its own")
	}

	done := make(chan struct{})
	go func() { e.stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop blocked on an already-exited tunnel")
	}
}

// A nil *exposeProc is the "no tunnel" case and must be safe to stop.
func TestExposeStopNil(t *testing.T) {
	var e *exposeProc
	e.stop()
}

// The host port defaults to the container port and is passed through otherwise.
func TestExposeArguments(t *testing.T) {
	for _, tc := range []struct {
		name     string
		hostPort string
		want     string
	}{
		{"default", "", "8765 8765"},
		{"explicit", "18765", "8765 18765"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := stubExposeOnPath(t, "echo \"$@\" > \"$(dirname \"$0\")/args\"\n")
			e, err := startBoothExpose(8765, tc.hostPort)
			if err != nil {
				t.Fatal(err)
			}
			defer e.stop()
			<-e.done

			got, err := os.ReadFile(filepath.Join(dir, "args"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(got)) != tc.want {
				t.Fatalf("args = %q, want %q", strings.TrimSpace(string(got)), tc.want)
			}
		})
	}
}
