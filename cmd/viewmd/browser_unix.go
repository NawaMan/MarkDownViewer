// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build !windows && !darwin

package main

import (
	"os"
	"strings"
)

// browserCommands lists the launchers to try, best first.
//
// $BROWSER leads because it is the user's own answer to the question. xdg-open
// honours it as well, but xdg-open is not always installed — a slim container
// or a bare WM often has none of these, which is why the list is long and the
// caller treats an empty result as a warning rather than a failure. wslview
// covers WSL, where the browser lives on the Windows side of the fence.
func browserCommands(url string) [][]string {
	var cmds [][]string
	for _, entry := range strings.Split(os.Getenv("BROWSER"), ":") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// The convention allows a %s placeholder for the URL.
		if strings.Contains(entry, "%s") {
			cmds = append(cmds, strings.Fields(strings.ReplaceAll(entry, "%s", url)))
			continue
		}
		cmds = append(cmds, append(strings.Fields(entry), url))
	}
	return append(cmds,
		[]string{"xdg-open", url},
		[]string{"gio", "open", url},
		[]string{"wslview", url},
		[]string{"sensible-browser", url},
		[]string{"x-www-browser", url},
		[]string{"www-browser", url},
	)
}
