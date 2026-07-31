# MarkDownViewer (`viewmd`)

Browse a folder of Markdown files in your browser — single Go binary, embedded UI, no Node install.

![status](https://img.shields.io/badge/status-early-blue)
[![CI](https://github.com/NawaMan/MarkDownViewer/actions/workflows/ci.yml/badge.svg)](https://github.com/NawaMan/MarkDownViewer/actions/workflows/ci.yml)

## Features

- Serve any directory of `*.md` / `*.markdown` files over HTTP
- Sidebar file tree + GitHub-flavoured rendering ([Marked](https://github.com/markedjs/marked))
- Resizable sidebar (width remembered), horizontal scroll, **Shift+wheel** for sideways scroll
- Open a default file with `--md`
- **Daemon mode**: `--daemon` / `--status` / `--stop`, with graceful shutdown
- Optional **CodingBooth** integration: `--expose` calls `booth--expose` when available

## Quick start

```bash
./build.sh
./viewmd --folder . --md README.md
# open http://127.0.0.1:8765/
```

## Usage

```text
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
                     (no-op warning if booth--expose is not on PATH)
  --daemon           Serve in the background and return to the shell
  --stop             Alias for the stop command
  --status           Alias for the status command
  --pidfile PATH     Pid file (default <tmp>/viewmd-<port>.pid)
  --logfile PATH     Daemon log file (default <tmp>/viewmd-<port>.log)
  --version
  -h, --help
```

Examples:

```bash
viewmd --folder ./docs --md intro.md
viewmd --md README.md --port 9000
viewmd --folder . --md README.md --expose          # host port = server port
viewmd --md README.md --expose 18765               # host 18765 → container port
```

## Daemon mode

`--daemon` re-execs `viewmd` detached from the terminal and returns as soon as
the port is bound, so a failure to start is still reported to your shell:

```bash
viewmd --folder ./docs --md intro.md --daemon
# viewmd v0.1.0 running in background (pid 12345)
#   url:      http://0.0.0.0:8765/
#   pid file: /tmp/viewmd-8765.pid
#   log file: /tmp/viewmd-8765.log

viewmd status          # exit 0 when running, 1 when not
viewmd stop            # SIGTERM + graceful drain, then removes the pid file
```

`stop` and `status` are also spelled `--stop` and `--status`; the command and
flag forms are interchangeable.

The pid and log paths are keyed by port, so instances on different ports are
independent — pass the same `--port` to `--status` / `--stop` that you started
with (or point all three at an explicit `--pidfile`):

```bash
viewmd --folder ./docs --port 9000 --daemon
viewmd stop --port 9000
```

Starting a second daemon against a pid file that is already live fails instead
of double-starting; a stale pid file (process gone) is cleaned up automatically.
Server output goes to the log file. On Windows `--stop` kills the process
rather than signalling it, since there is no SIGTERM delivery.

## Build

```bash
./build.sh           # ./viewmd for this machine
./build.sh --all     # also bin/viewmd-<os>-<arch>
```

Requires Go 1.24+.

## Releases

CI (`.github/workflows/ci.yml`) vets and formats on Linux, runs the tests on
Linux/macOS/Windows plus a race-detector pass, and cross-compiles every target
on each push and pull request.

To cut a release, bump `version.txt` and push a matching tag:

```bash
echo "0.2.0" > version.txt
git commit -am "Release 0.2.0"
git tag v0.2.0
git push origin main v0.2.0
```

`.github/workflows/release.yml` then tests, builds all five targets, and
publishes a GitHub release with the binaries and a `SHA256SUMS` file. The tag
must match `version.txt` (`v0.2.0` ↔ `0.2.0`) or the workflow fails before
publishing anything, and the built binary's `viewmd version` output is checked
against the tag as well.

The release can also be run manually from the Actions tab. It takes no inputs:
the tag is derived from `version.txt` (`0.2.0` → `v0.2.0`), and the workflow
checks out that tag so it builds the tagged commit rather than whatever the
dispatch ref points at. It fails with a clear message if the tag does not exist
yet.

## Layout

```text
cmd/viewmd/          Go command (HTTP server, path jail, tree walk, daemon)
cmd/viewmd/web/      Embedded UI (index.html + vendor/marked.umd.js)
build.sh
version.txt
```

## Origin

Extracted from the [CodingBooth](https://github.com/nawaman/codingbooth) experiment as a standalone tool.

## License

- Go code and UI shell: Apache License 2.0 (see `LICENSE`)
- Vendored Marked (`cmd/viewmd/web/vendor/marked.umd.js`): MIT (MarkedJS)
