// Copyright 2025-2026 : Nawa Manusitthipol
// Licensed under the Apache License, Version 2.0 (the "License");

// viewmd serves a Markdown directory tree over HTTP with an embedded viewer.
//
//	viewmd [--folder DIR] [--port N] [--bind ADDR] [--md FILE] [--expose [HOSTPORT]]
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

//go:embed all:web
var webRoot embed.FS

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// A leading bare word is a subcommand; flags may still follow it, as in
	// `viewmd stop --port 9000`.
	var command string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
		switch command {
		case "version", "stop", "status":
		default:
			fmt.Fprintf(os.Stderr, "Error: unknown command %q (try: version, stop, status)\n", command)
			return 2
		}
	}

	// version needs no flags and no valid folder.
	if command == "version" {
		fmt.Println(version)
		return 0
	}

	flags := flag.NewFlagSet("viewmd", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	folder := flags.String("folder", ".", "directory whose Markdown files to serve")
	port := flags.Int("port", 8765, "HTTP listen port")
	bind := flags.String("bind", "0.0.0.0", "listen address")
	md := flags.String("md", "", "Markdown file (relative to --folder) to open first")
	serverOnly := flags.Bool("server-only", false, "serve without opening a browser")
	daemon := flags.Bool("daemon", false, "run in the background and return to the shell")
	stop := flags.Bool("stop", false, "stop the background instance for this port")
	status := flags.Bool("status", false, "report whether a background instance is running")
	pidFile := flags.String("pidfile", "", "pid file path (default <tmp>/viewmd-<port>.pid)")
	logFile := flags.String("logfile", "", "daemon log path (default <tmp>/viewmd-<port>.log)")
	// --expose is parsed separately so the host port is optional.

	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, `viewmd — browse Markdown files in a folder (embedded UI)

Usage:
  viewmd [flags]
  viewmd <command> [flags]

Commands:
  version            Print version and exit (same as --version)
  stop               Stop the background instance for --port (same as --stop)
  status             Report whether a background instance is running

Flags:
  --folder DIR       Root directory to scan (default ".")
  --port N           Listen port (default 8765)
  --bind ADDR        Listen address (default 0.0.0.0)
  --md FILE          Open this Markdown file first (relative to --folder)
  --expose [PORT]    After listen, run booth--expose <port> [PORT]
                     Host port defaults to the server port when omitted
  --server-only      Do not open a browser (the default is to open one)
  --daemon           Serve in the background and return to the shell
  --stop             Alias for the stop command
  --status           Alias for the status command
  --pidfile PATH     Pid file (default <tmp>/viewmd-<port>.pid)
  --logfile PATH     Daemon log file (default <tmp>/viewmd-<port>.log)
  --version          Print version and exit
  -h, --help         Show this help

Examples:
  viewmd --folder . --md README.md
  viewmd --folder . --md README.md --server-only   # no browser (headless, CI)
  viewmd --folder docs --port 8765 --expose
  viewmd --md README.md --expose 18765
  viewmd --folder ./docs --daemon        # background; --port picks the instance
  viewmd status
  viewmd stop --port 9000
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

	pidPath := *pidFile
	if pidPath == "" {
		pidPath = defaultPidFile(*port)
	}
	logPath := *logFile
	if logPath == "" {
		logPath = defaultLogFile(*port)
	}

	// stop / status act on an existing instance; they need no valid folder.
	stopReq := *stop || command == "stop"
	statusReq := *status || command == "status"
	if stopReq && statusReq {
		fmt.Fprintln(os.Stderr, "Error: stop and status are mutually exclusive")
		return 2
	}
	if stopReq {
		pid, err := stopDaemon(pidPath, stopTimeout)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: stop:", err)
			return 1
		}
		if pid == 0 {
			fmt.Printf("viewmd: not running (%s)\n", pidPath)
			return 0
		}
		fmt.Printf("viewmd: stopped pid %d\n", pid)
		return 0
	}
	if statusReq {
		pid, err := runningPid(pidPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: status:", err)
			return 1
		}
		if pid == 0 {
			fmt.Printf("viewmd: not running (%s)\n", pidPath)
			return 1
		}
		fmt.Printf("viewmd: running (pid %d, port %d)\n  pid file: %s\n  log file: %s\n",
			pid, *port, pidPath, logPath)
		return 0
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

	// Validation above runs in the foreground so bad arguments are reported to
	// the terminal rather than buried in the daemon log.
	daemonChild := os.Getenv(daemonEnv) != ""
	if *daemon && !daemonChild {
		pid, err := spawnDaemon(pidPath, logPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: daemon:", err)
			return 1
		}
		fmt.Printf("viewmd v%s running in background (pid %d)\n", version, pid)
		fmt.Printf("  url:      %s\n", browsableURL(*bind, *port))
		if note := bindNote(*bind); note != "" {
			fmt.Printf("  listen:   %s (%s)\n", net.JoinHostPort(*bind, strconv.Itoa(*port)), note)
		}
		fmt.Printf("  pid file: %s\n", pidPath)
		fmt.Printf("  log file: %s\n", logPath)
		fmt.Printf("  stop it:  viewmd stop --port %d\n", *port)
		// spawnDaemon returns only once the child has bound the port, so the
		// browser cannot beat the listener to the first request.
		if !*serverOnly {
			announceBrowser(*bind, *port, "opening:  ") // padded to this banner's keys
		}
		return 0
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

	// The pid file is written once the port is bound, which is what tells the
	// spawning parent that startup succeeded.
	if daemonChild || *pidFile != "" {
		if err := writePidFile(pidPath, os.Getpid()); err != nil {
			fmt.Fprintln(os.Stderr, "Error: pid file:", err)
			_ = ln.Close()
			return 1
		}
		defer os.Remove(pidPath)
	}

	fmt.Fprintf(os.Stderr, "viewmd v%s serving %s on %s\n", version, rootAbs, browsableURL(*bind, *port))
	if note := bindNote(*bind); note != "" {
		fmt.Fprintf(os.Stderr, "  listen:       %s (%s)\n", addr, note)
	}
	if initial != "" {
		fmt.Fprintf(os.Stderr, "  initial file: %s\n", initial)
	}

	// The port is bound by now, so the browser cannot arrive early. A daemon
	// child never opens one: it has no terminal and no session of its own, and
	// the parent that spawned it has already done this.
	if !*serverOnly && !daemonChild {
		announceBrowser(*bind, *port, "opening:      ") // padded to this banner's keys
	}

	// booth--expose may be a long-lived tunnel, so it runs alongside the server
	// rather than ahead of it, and is torn down when the server stops.
	var expose *exposeProc
	if exposeSet {
		p, err := startBoothExpose(*port, exposeHost)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: --expose failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "  (Is booth--expose on PATH? Continuing without host tunnel.)")
		} else {
			expose = p
		}
	}
	defer expose.stop()

	// SIGTERM (from --stop) and Ctrl+C both drain in-flight requests first.
	shutdown := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "viewmd: %v received, shutting down\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
		expose.stop()
		close(shutdown)
	}()

	if err := httpSrv.Serve(ln); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "Error: server:", err)
			return 1
		}
		<-shutdown
	}
	return 0
}

// announceBrowser opens the viewer and reports — rather than fails — when
// there is no browser to open. A headless box, a container or an SSH session
// is a normal place to run viewmd, and the server is useful there regardless.
// The label carries its own padding: the two banners align their keys to
// different columns.
func announceBrowser(bind string, port int, label string) {
	url := browsableURL(bind, port)
	if err := openBrowser(url); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not open a browser: %v\n", err)
		fmt.Fprintf(os.Stderr, "  (Open %s yourself, or pass --server-only to stop trying.)\n", url)
		return
	}
	fmt.Fprintf(os.Stderr, "  %s%s\n", label, url)
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

// exposeProc tracks the booth--expose child so that it never blocks the HTTP
// server and does not outlive it. A nil *exposeProc is a usable no-op.
type exposeProc struct {
	cmd      *exec.Cmd
	stopping atomic.Bool
	done     chan struct{}
}

// startBoothExpose launches booth--expose in the background. It reports an
// error only when the process could not be started at all; a later non-zero
// exit is surfaced as a warning by the watcher goroutine.
func startBoothExpose(containerPort int, hostPort string) (*exposeProc, error) {
	bin, err := exec.LookPath("booth--expose")
	if err != nil {
		return nil, fmt.Errorf("booth--expose not found: %w", err)
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
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	e := &exposeProc{cmd: cmd, done: make(chan struct{})}
	go func() {
		defer close(e.done)
		// Wait reaps the child; a failure we did not cause is worth reporting.
		if err := cmd.Wait(); err != nil && !e.stopping.Load() {
			fmt.Fprintf(os.Stderr, "Warning: booth--expose exited: %v\n", err)
			fmt.Fprintln(os.Stderr, "  (Continuing without host tunnel.)")
		}
	}()
	return e, nil
}

// stop terminates the tunnel and waits briefly for it to exit. Safe to call
// on a nil receiver and more than once.
func (e *exposeProc) stop() {
	if e == nil || e.cmd.Process == nil {
		return
	}
	if !e.stopping.CompareAndSwap(false, true) {
		return
	}
	select {
	case <-e.done: // already exited on its own
		return
	default:
	}
	_ = terminateProcess(e.cmd.Process)
	select {
	case <-e.done:
	case <-time.After(3 * time.Second):
		_ = e.cmd.Process.Kill()
	}
}
