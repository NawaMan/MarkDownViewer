// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build !windows

package main

import (
	"errors"
	"os"
	"syscall"
)

// detachAttr puts the child in its own session so it survives the parent's
// exit and is detached from the controlling terminal.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// processAlive probes the process with signal 0. EPERM means it exists but
// belongs to another user, which still counts as alive.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

func terminateProcess(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
