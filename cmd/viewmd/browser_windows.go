// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

//go:build windows

package main

// browserCommands lists the launchers to try, best first.
//
// rundll32 rather than `cmd /c start`: start reads a lone quoted argument as a
// window title, so it needs a dummy one, and cmd splits a URL on `&`. rundll32
// is in System32, which is always on PATH, and takes the URL verbatim.
func browserCommands(url string) [][]string {
	return [][]string{{"rundll32", "url.dll,FileProtocolHandler", url}}
}
