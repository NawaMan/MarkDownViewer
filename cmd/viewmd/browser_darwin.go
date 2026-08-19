// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build darwin

package main

// browserCommands lists the launchers to try, best first. macOS has one, and
// `open` hands the URL to whatever the user set as their default browser.
func browserCommands(url string) [][]string {
	return [][]string{{"open", url}}
}
