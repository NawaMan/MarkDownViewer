# MarkDownViewer (`viewmd`)

Browse a folder of Markdown files in your browser — single Go binary, embedded UI, no Node install.

![status](https://img.shields.io/badge/status-early-blue)
[![CI](https://github.com/NawaMan/MarkDownViewer/actions/workflows/ci.yml/badge.svg)](https://github.com/NawaMan/MarkDownViewer/actions/workflows/ci.yml)

## Features

- Serve any directory of `*.md` / `*.markdown` files over HTTP
- Sidebar file tree + GitHub-flavoured rendering ([Marked](https://github.com/markedjs/marked))
- Resizable sidebar (width remembered), horizontal scroll, **Shift+wheel** for sideways scroll
- Copy button on every code block (falls back to a selection copy off localhost)
- Open a default file with `--md`
- **Daemon mode**: `--daemon` / `--status` / `--stop`, with graceful shutdown
- Optional **CodingBooth** integration: `--expose` calls `booth--expose` when available

## Quick start

No Go, no Node. Download the binary for your machine:

### Install Script

On Bash/Zssh
```bash
curl -fsSL https://github.com/NawaMan/MarkDownViewer/releases/latest/download/install.sh | sh
```

Or, on PowerShell
```powershell
irm https://github.com/NawaMan/MarkDownViewer/releases/latest/download/install.ps1 | iex
```

It lands in `/usr/local/bin` when that is writable and `~/.local/bin` otherwise
(`%LOCALAPPDATA%\Programs\viewmd` on Windows, which is added to the user PATH).
Two knobs, both environment variables: `VIEWMD_INSTALL_DIR` picks the
directory, `VIEWMD_VERSION` picks a tag other than the newest. An installer
fetched from a pinned release installs that release, not the newest one.

Run the file directly

**BASH or ZSH**
```bash
chmod +x viewmd-linux-amd64
./viewmd-linux-amd64 --folder . --md README.md
```

**PowerShell**
```powershell
.\viewmd-windows-amd64.exe --folder . --md README.md
```

Then open : http://127.0.0.1:8765/

### Download the File Directly

|  | Linux | macOS | Windows |
| --- | --- | --- | --- |
| **x86-64** | [viewmd&#8209;linux&#8209;amd64](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-linux-amd64) | [viewmd&#8209;darwin&#8209;amd64](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-darwin-amd64) | [viewmd&#8209;windows&#8209;amd64.exe](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-windows-amd64.exe) |
| **ARM64** | [viewmd&#8209;linux&#8209;arm64](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-linux-arm64) | [viewmd&#8209;darwin&#8209;arm64](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-darwin-arm64) | — |

Then run it — Linux and macOS need the executable bit first:

macOS blocks binaries downloaded by a browser until they are cleared:
`xattr -d com.apple.quarantine viewmd-darwin-arm64`.

Verify a download against the checksums file (keep the original asset name so
the entries match; on macOS use `shasum -a 256 -c`):

```bash
curl -LO https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-linux-amd64
curl -LO https://github.com/NawaMan/MarkDownViewer/releases/latest/download/SHA256SUMS
sha256sum -c SHA256SUMS --ignore-missing
chmod +x viewmd-linux-amd64 && ./viewmd-linux-amd64 version
```

Note the path order: `releases/latest/download/<file>`, not
`releases/download/latest/<file>` — the latter looks for a tag named `latest`,
which does not exist.

### Build from Source

```bash
go install github.com/NawaMan/MarkDownViewer/cmd/viewmd@latest
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
# viewmd v0.3.1 running in background (pid 12345)
#   url:      http://0.0.0.0:8765/
#   pid file: /tmp/viewmd-8765.pid
#   log file: /tmp/viewmd-8765.log
#   stop it:  viewmd stop --port 8765

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

Requires Go 1.24+. Builds are pure Go (`CGO_ENABLED=0`), so the Linux binaries
are static and run on musl (Alpine) and distroless images too.

## Releases

CI (`.github/workflows/ci.yml`) vets and formats on Linux, runs the tests on
Linux/macOS/Windows plus a race-detector pass, and then hands off to
`binaries.yml`: it builds all five targets once and *runs* each one on a native
runner of its own platform — Linux x86-64 and ARM64, macOS Intel and Apple
Silicon, Windows — serving a document over HTTP and reading it back. The Linux
x86-64 binary additionally has to start inside Alpine, which has no glibc.

`Release` reuses that same workflow and publishes only what passed it.

To cut a release, bump `version.txt` and push a matching tag:

```bash
echo "0.2.0" > version.txt
git commit -am "Release 0.2.0"
git tag v0.2.0
git push origin main v0.2.0
```

`.github/workflows/release.yml` then tests, builds all five targets, and
publishes a GitHub release with the binaries, both installers, and a
`SHA256SUMS` file covering all of them. The tag must match `version.txt`
(`v0.2.0` ↔ `0.2.0`) or the workflow fails before publishing anything; the
built binary's `viewmd version` output is checked against the tag, and after
publishing, the release's own `install.sh` is run and its result checked too.

The release can also be run manually from the Actions tab. There is no tag to
enter: it is derived from `version.txt` (`0.2.0` → `v0.2.0`), and the workflow
checks out that tag so it builds the tagged commit rather than whatever the
dispatch ref points at. It fails with a clear message if the tag does not exist
yet.

If a release for the tag already exists the run stops rather than overwriting
it. Tick **force** on a manual run to replace it instead; that deletes and
recreates the release, leaving the git tag itself alone.

A release is marked "Latest" — the target of the `releases/latest` redirect —
only when its version is the highest published one, so re-releasing an older
tag cannot displace a newer one. Pre-release tags (`v1.2.3-rc1`) are never
marked Latest.

## Layout

```text
cmd/viewmd/          Go command (HTTP server, path jail, tree walk, daemon)
cmd/viewmd/web/      Embedded UI (index.html + vendor/marked.umd.js)
build.sh
install.sh          One-line installer (Linux/macOS), published per release
install.ps1         Same for Windows
version.txt
```

## Origin

Extracted from the [CodingBooth](https://github.com/nawaman/codingbooth) experiment as a standalone tool.

## License

- Go code and UI shell: Apache License 2.0 (see `LICENSE`)
- Vendored Marked (`cmd/viewmd/web/vendor/marked.umd.js`): MIT (MarkedJS)
