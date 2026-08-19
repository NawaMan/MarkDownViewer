// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// browsableHost turns a listen address into one a browser can connect to.
// The default bind is 0.0.0.0, which means "every interface" to a listener and
// nothing useful to a client — Windows refuses to connect to it at all.
func browsableHost(bind string) string {
	host := strings.Trim(bind, "[]")
	if host == "" {
		return "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsUnspecified() {
		return host // a hostname or a real address: hand it over as given
	}
	if ip.To4() != nil {
		return "127.0.0.1"
	}
	return "::1"
}

// browsableURL is the address to hand a browser, given what the server bound.
func browsableURL(bind string, port int) string {
	return "http://" + net.JoinHostPort(browsableHost(bind), strconv.Itoa(port)) + "/"
}

// bindNote describes what a browsable URL leaves out. Rewriting 0.0.0.0 to
// 127.0.0.1 makes the URL usable and hides the part that matters for a bind
// that answers on every interface: the machine is reachable from off-box.
func bindNote(bind string) string {
	host := strings.Trim(bind, "[]")
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		return "all interfaces"
	}
	return ""
}

// openBrowser points the user's default browser at url. It returns once a
// launcher has started, not once the page is up.
func openBrowser(url string) error {
	return openBrowserWith(browserCommands(url))
}

// openBrowserWith tries each launcher in turn and keeps the first that starts.
// Only a failure to start moves on to the next one: a launcher hands off to the
// browser and exits, so its status arrives long after the choice has to be made.
func openBrowserWith(candidates [][]string) error {
	if len(candidates) == 0 {
		return errors.New("no browser launcher is known for this platform")
	}
	var firstErr error
	for _, argv := range candidates {
		cmd := exec.Command(argv[0], argv[1:]...)
		// Launchers chatter; the server's own output owns the terminal.
		cmd.Stdout, cmd.Stderr = nil, nil
		if err := cmd.Start(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		go func() {
			// Reap it. A non-zero exit means the launcher was there but could
			// not do the job — no display, no registered browser — and the user
			// can only act on that if it is said out loud.
			if err := cmd.Wait(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s exited: %v\n", argv[0], err)
				fmt.Fprintln(os.Stderr, "  (No browser opened. The server is still running.)")
			}
		}()
		return nil
	}
	return firstErr
}
