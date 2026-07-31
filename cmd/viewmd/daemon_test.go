// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPidFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "viewmd.pid")
	if err := writePidFile(path, 4242); err != nil {
		t.Fatal(err)
	}
	pid, err := readPidFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 4242 {
		t.Fatalf("pid = %d, want 4242", pid)
	}
}

func TestReadPidFileRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "viewmd.pid")
	if err := os.WriteFile(path, []byte("not-a-pid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPidFile(path); err == nil {
		t.Fatal("expected error for non-numeric pid file")
	}
}

func TestRunningPidStates(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing.pid")
	if pid, err := runningPid(missing); err != nil || pid != 0 {
		t.Fatalf("missing: pid=%d err=%v, want 0/nil", pid, err)
	}

	// A pid that is almost certainly not in use: stale file, not an error.
	stale := filepath.Join(dir, "stale.pid")
	if err := writePidFile(stale, 0x7FFFFFF0); err != nil {
		t.Fatal(err)
	}
	if pid, err := runningPid(stale); err != nil || pid != 0 {
		t.Fatalf("stale: pid=%d err=%v, want 0/nil", pid, err)
	}

	live := filepath.Join(dir, "live.pid")
	if err := writePidFile(live, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if pid, err := runningPid(live); err != nil || pid != os.Getpid() {
		t.Fatalf("live: pid=%d err=%v, want %d/nil", pid, err, os.Getpid())
	}
}

func TestStopDaemonNotRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.pid")
	if err := writePidFile(path, 0x7FFFFFF0); err != nil {
		t.Fatal(err)
	}
	pid, err := stopDaemon(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 0 {
		t.Fatalf("pid = %d, want 0 for a stale pid file", pid)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stale pid file should have been removed")
	}
}

// TestHelperSleep is not a real test: the tests below re-exec the test binary
// with VIEWMD_TEST_HELPER set to get a long-running child process.
func TestHelperSleep(t *testing.T) {
	if os.Getenv("VIEWMD_TEST_HELPER") != "1" {
		t.Skip("helper process, not a test")
	}
	time.Sleep(60 * time.Second)
}

type helper struct {
	pid  int
	done chan struct{}
}

// helperProcess starts a child that lives until it is signalled.
//
// The reaper goroutine matters: a signalled child of this process stays a
// zombie until it is waited for, and a zombie still answers signal 0, so
// processAlive would never report it gone. Nothing reaps it in these tests
// otherwise — unlike in real use, where the daemon is never the child of the
// process running stop/status.
func helperProcess(t *testing.T) *helper {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperSleep$")
	cmd.Env = append(os.Environ(), "VIEWMD_TEST_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	h := &helper{pid: cmd.Process.Pid, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(h.done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill() // no-op once the process has been reaped
		<-h.done
	})
	return h
}

// stopDaemon must signal a live process, wait for it, and clear the pid file.
func TestStopDaemonTerminatesLiveProcess(t *testing.T) {
	child := helperProcess(t)
	pidPath := filepath.Join(t.TempDir(), "viewmd.pid")
	if err := writePidFile(pidPath, child.pid); err != nil {
		t.Fatal(err)
	}

	pid, err := stopDaemon(pidPath, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if pid != child.pid {
		t.Fatalf("stopped pid = %d, want %d", pid, child.pid)
	}
	if processAlive(child.pid) {
		t.Fatal("child still alive after stopDaemon")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("pid file should have been removed")
	}
}

// runningPid reports a live process started outside this one.
func TestRunningPidSeesLiveChild(t *testing.T) {
	child := helperProcess(t)
	pidPath := filepath.Join(t.TempDir(), "viewmd.pid")
	if err := writePidFile(pidPath, child.pid); err != nil {
		t.Fatal(err)
	}
	pid, err := runningPid(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	if pid != child.pid {
		t.Fatalf("runningPid = %d, want %d", pid, child.pid)
	}
}

// A pid file holding a live process blocks a second daemon from starting.
func TestSpawnDaemonRefusesWhenAlreadyRunning(t *testing.T) {
	child := helperProcess(t)
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "viewmd.pid")
	if err := writePidFile(pidPath, child.pid); err != nil {
		t.Fatal(err)
	}
	if _, err := spawnDaemon(pidPath, filepath.Join(dir, "viewmd.log")); err == nil {
		t.Fatal("expected spawnDaemon to refuse while an instance is running")
	} else if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("error = %v, want it to mention \"already running\"", err)
	}
}

func TestDefaultPathsArePortKeyed(t *testing.T) {
	if a, b := defaultPidFile(8765), defaultPidFile(9000); a == b {
		t.Fatalf("pid files collide across ports: %s", a)
	}
	if !strings.HasSuffix(defaultLogFile(8765), "viewmd-8765.log") {
		t.Fatalf("unexpected log file name: %s", defaultLogFile(8765))
	}
}

// TestDaemonLifecycle builds the binary and exercises --daemon, --status and
// --stop end to end.
func TestDaemonLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and spawns a process")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "viewmd")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	pidPath := filepath.Join(dir, "viewmd.pid")
	logPath := filepath.Join(dir, "viewmd.log")
	base := []string{
		"--folder", root, "--bind", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--pidfile", pidPath, "--logfile", logPath,
	}

	out, err := exec.Command(bin, append([]string{"--daemon"}, base...)...).CombinedOutput()
	if err != nil {
		logs, _ := os.ReadFile(logPath)
		t.Fatalf("--daemon failed: %v\n%s\nlog:\n%s", err, out, logs)
	}
	t.Cleanup(func() { _ = exec.Command(bin, append([]string{"--stop"}, base...)...).Run() })

	pid, err := runningPid(pidPath)
	if err != nil || pid == 0 {
		t.Fatalf("no live pid after --daemon: pid=%d err=%v\n%s", pid, err, out)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/config", port))
	if err != nil {
		t.Fatalf("daemon not serving: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	if out, err := exec.Command(bin, append([]string{"--status"}, base...)...).CombinedOutput(); err != nil {
		t.Fatalf("--status failed: %v\n%s", err, out)
	}

	// A second --daemon on the same pid file must refuse rather than double-start.
	if out, err := exec.Command(bin, append([]string{"--daemon"}, base...)...).CombinedOutput(); err == nil {
		t.Fatalf("second --daemon should have failed, got:\n%s", out)
	}

	if out, err := exec.Command(bin, append([]string{"--stop"}, base...)...).CombinedOutput(); err != nil {
		t.Fatalf("--stop failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("pid file should be gone after --stop")
	}
	if err := exec.Command(bin, append([]string{"--status"}, base...)...).Run(); err == nil {
		t.Fatal("--status should exit non-zero when not running")
	}
}

// TestExposeDoesNotBlockServer starts the daemon with a booth--expose stub that
// never exits, and checks that the server still serves and that the stub is
// terminated when the server stops.
func TestExposeDoesNotBlockServer(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and spawns processes")
	}
	if runtime.GOOS == "windows" {
		t.Skip("stub tunnel is a shell script")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "viewmd")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// A tunnel that records its pid and arguments, then blocks forever.
	stubDir := filepath.Join(dir, "stub")
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	argsPath := filepath.Join(dir, "expose.args")
	pidStubPath := filepath.Join(dir, "expose.pid")
	stub := fmt.Sprintf("#!/bin/sh\necho \"$@\" > %q\necho $$ > %q\nwhile true; do sleep 1; done\n", argsPath, pidStubPath)
	if err := os.WriteFile(filepath.Join(stubDir, "booth--expose"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	pidPath := filepath.Join(dir, "viewmd.pid")
	base := []string{
		"--folder", root, "--bind", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--pidfile", pidPath, "--logfile", filepath.Join(dir, "viewmd.log"),
	}

	start := exec.Command(bin, append([]string{"--daemon", "--expose"}, base...)...)
	start.Env = append(os.Environ(), "PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("--daemon --expose failed: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command(bin, append([]string{"--stop"}, base...)...).Run() })

	// The blocking tunnel must not have prevented the server from serving.
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/config", port))
	if err != nil {
		t.Fatalf("server not reachable while booth--expose runs: %v", err)
	}
	resp.Body.Close()

	gotArgs, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("booth--expose was not invoked: %v", err)
	}
	wantArgs := fmt.Sprintf("%d %d", port, port)
	if strings.TrimSpace(string(gotArgs)) != wantArgs {
		t.Fatalf("booth--expose args = %q, want %q", strings.TrimSpace(string(gotArgs)), wantArgs)
	}

	raw, err := os.ReadFile(pidStubPath)
	if err != nil {
		t.Fatal(err)
	}
	stubPid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}

	if out, err := exec.Command(bin, append([]string{"--stop"}, base...)...).CombinedOutput(); err != nil {
		t.Fatalf("--stop failed: %v\n%s", err, out)
	}
	// The tunnel should not outlive the server.
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(stubPid) {
		if time.Now().After(deadline) {
			t.Fatalf("booth--expose (pid %d) still running after --stop", stubPid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
