// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build !windows

package main

import (
	"bytes"
	"errors"
	"os"
	"strconv"
	"syscall"
)

// detachAttr puts the child in its own session so it survives the parent's
// exit and is detached from the controlling terminal.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// processAlive probes the process with signal 0. EPERM means it exists but
// belongs to another user, which still counts as alive.
//
// A zombie answers signal 0 as happily as a running process, so the probe
// alone cannot tell "still serving" from "exited, nobody reaped it yet". That
// is not a corner case for a daemon: once its spawner exits, the daemon is
// re-parented to pid 1, and a pid 1 that is a shell — the normal shape of a
// container — reaps only when it happens to wait. Without the /proc check,
// `viewmd stop` sits out its whole timeout and then reports a failure for a
// process that already exited. Systems without /proc (macOS) fall back to the
// probe, where init reaps orphans immediately anyway.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	return !processReaped(pid)
}

// processReaped reports whether the pid names a zombie: exited, still in the
// table only because its parent has not collected it. False when the state
// cannot be read, so an unreadable /proc never turns a live process into a
// dead one.
func processReaped(pid int) bool {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	// "pid (comm) state ..." — comm is arbitrary and may hold spaces or
	// parentheses, so the state field is found from the last ')'.
	commEnd := bytes.LastIndexByte(stat, ')')
	if commEnd < 0 || commEnd+2 >= len(stat) {
		return false
	}
	return stat[commEnd+2] == 'Z'
}

func terminateProcess(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
