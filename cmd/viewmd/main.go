// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

// viewmd serves a Markdown directory tree over HTTP with an embedded viewer.
//
//	viewmd [--folder DIR] [--port N] [--bind ADDR] [--md FILE] [--expose [HOSTPORT]]
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed all:web
var webRoot embed.FS

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("viewmd", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	folder := flags.String("folder", ".", "directory whose Markdown files to serve")
	port := flags.Int("port", 8765, "HTTP listen port")
	bind := flags.String("bind", "0.0.0.0", "listen address")
	md := flags.String("md", "", "Markdown file (relative to --folder) to open first")
	// --expose is parsed separately so the host port is optional.

	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, `viewmd — browse Markdown files in a folder (embedded UI)

Usage:
  viewmd [flags]

Flags:
  --folder DIR       Root directory to scan (default ".")
  --port N           Listen port (default 8765)
  --bind ADDR        Listen address (default 0.0.0.0)
  --md FILE          Open this Markdown file first (relative to --folder)
  --expose [PORT]    After listen, run booth--expose <port> [PORT]
                     Host port defaults to the server port when omitted
  --version          Print version and exit
  -h, --help         Show this help

Examples:
  viewmd --folder . --md README.md
  viewmd --folder docs --port 8765 --expose
  viewmd --md README.md --expose 18765
`)
	}

	exposeSet, exposeHost, rest, err := peelExpose(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 2
	}

	// Accept --version before flag parse
	for _, a := range rest {
		if a == "--version" || a == "-version" {
			fmt.Println(version)
			return 0
		}
	}

	if err := flags.Parse(rest); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments: %v\n", flags.Args())
		return 2
	}

	rootAbs, err := filepath.Abs(*folder)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: folder:", err)
		return 1
	}
	st, err := os.Stat(rootAbs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: folder:", err)
		return 1
	}
	if !st.IsDir() {
		fmt.Fprintln(os.Stderr, "Error: --folder is not a directory")
		return 1
	}

	initial, err := normalizeInitialMd(rootAbs, *md)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}

	sub, err := fs.Sub(webRoot, "web")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: embed:", err)
		return 1
	}

	srv := &viewServer{
		root:      rootAbs,
		folderArg: *folder,
		initialMd: initial,
		port:      *port,
		web:       sub,
	}

	addr := net.JoinHostPort(*bind, strconv.Itoa(*port))
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: listen:", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "viewmd v%s serving %s on http://%s/\n", version, rootAbs, addr)
	if initial != "" {
		fmt.Fprintf(os.Stderr, "  initial file: %s\n", initial)
	}

	if exposeSet {
		if err := runBoothExpose(*port, exposeHost); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: --expose failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "  (Is booth--expose on PATH? Continuing without host tunnel.)")
		}
	}

	if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "Error: server:", err)
		return 1
	}
	return 0
}

// peelExpose extracts --expose / --expose=PORT / --expose PORT from args.
// When present without a value, host port is "" (same as server port).
func peelExpose(args []string) (set bool, hostPort string, rest []string, err error) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--expose":
			set = true
			// Optional next arg if it does not look like a flag
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				hostPort = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--expose="):
			set = true
			hostPort = strings.TrimPrefix(a, "--expose=")
		default:
			rest = append(rest, a)
		}
	}
	if set && hostPort != "" {
		// allow +OFFSET or digits
		if hostPort[0] == '+' {
			if _, e := strconv.Atoi(hostPort[1:]); e != nil {
				return false, "", nil, fmt.Errorf("invalid --expose port %q", hostPort)
			}
		} else if _, e := strconv.Atoi(hostPort); e != nil {
			return false, "", nil, fmt.Errorf("invalid --expose port %q", hostPort)
		}
	}
	return set, hostPort, rest, nil
}

func runBoothExpose(containerPort int, hostPort string) error {
	bin, err := exec.LookPath("booth--expose")
	if err != nil {
		return fmt.Errorf("booth--expose not found: %w", err)
	}
	args := []string{strconv.Itoa(containerPort)}
	if hostPort != "" {
		args = append(args, hostPort)
	} else {
		// Same as server port (booth--expose default is also same, but be explicit)
		args = append(args, strconv.Itoa(containerPort))
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Fprintf(os.Stderr, "  running: booth--expose %s\n", strings.Join(args, " "))
	return cmd.Run()
}
