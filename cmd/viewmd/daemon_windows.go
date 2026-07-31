// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build windows

package main

import (
	"os"
	"syscall"
)

const (
	createNewProcessGroup          = 0x00000200
	createNoWindow                 = 0x08000000
	processQueryLimitedInformation = 0x00001000
	stillActive                    = 259
)

// detachAttr starts the child in its own process group without a console
// window, so it keeps running after this process exits.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | createNoWindow,
		HideWindow:    true,
	}
}

func processAlive(pid int) bool {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}

// Windows has no SIGTERM delivery for arbitrary processes; Kill is the
// available equivalent.
func terminateProcess(p *os.Process) error {
	return p.Kill()
}
