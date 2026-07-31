// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// daemonEnv marks a re-exec'd child so it serves in the foreground instead of
// spawning yet another background process.
const daemonEnv = "VIEWMD_DAEMONIZED"

// startupTimeout is how long the parent waits for a background child to bind
// its port and write the pid file.
const startupTimeout = 5 * time.Second

// stopTimeout is how long --stop waits for a graceful shutdown.
const stopTimeout = 5 * time.Second

// defaultPidFile and defaultLogFile are keyed by port so several instances can
// run side by side without extra flags.
func defaultPidFile(port int) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("viewmd-%d.pid", port))
}

func defaultLogFile(port int) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("viewmd-%d.log", port))
}

func writePidFile(path string, pid int) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

// readPidFile returns the recorded pid; a missing file yields os.ErrNotExist.
func readPidFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid file %s", path)
	}
	return pid, nil
}

// runningPid reports the pid in path when that process is still alive.
// A missing or stale pid file reports 0 with no error.
func runningPid(path string) (int, error) {
	pid, err := readPidFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !processAlive(pid) {
		return 0, nil
	}
	return pid, nil
}

// spawnDaemon re-execs this binary in the background with the same arguments
// and waits until the child reports ready by writing pidPath.
func spawnDaemon(pidPath, logPath string) (int, error) {
	if pid, err := runningPid(pidPath); err != nil {
		return 0, err
	} else if pid > 0 {
		return 0, fmt.Errorf("already running (pid %d, %s)", pid, pidPath)
	}

	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	if dir := filepath.Dir(logPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
	}
	logF, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("log file: %w", err)
	}
	defer logF.Close()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), daemonEnv+"=1")
	cmd.Stdin = devNull
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.SysProcAttr = detachAttr()
	if err := cmd.Start(); err != nil {
		return 0, err
	}

	// Reap the child if it dies during startup so the wait below sees it.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			return 0, fmt.Errorf("daemon exited during startup (%v); see %s", err, logPath)
		default:
		}
		if pid, err := runningPid(pidPath); err == nil && pid > 0 {
			return pid, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return 0, fmt.Errorf("daemon did not become ready within %s; see %s", startupTimeout, logPath)
}

// stopDaemon signals the recorded process and waits for it to exit. It returns
// the pid that was stopped, or 0 when nothing was running.
func stopDaemon(pidPath string, timeout time.Duration) (int, error) {
	pid, err := runningPid(pidPath)
	if err != nil {
		return 0, err
	}
	if pid == 0 {
		_ = os.Remove(pidPath) // clean up a stale file if there was one
		return 0, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, err
	}
	if err := terminateProcess(proc); err != nil {
		return pid, fmt.Errorf("signal pid %d: %w", pid, err)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			_ = os.Remove(pidPath)
			return pid, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return pid, fmt.Errorf("pid %d did not exit within %s", pid, timeout)
}
