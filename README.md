# MarkDownViewer (`viewmd`)

Browse a folder of Markdown files in your browser — single Go binary, embedded UI, no Node install.

![status](https://img.shields.io/badge/status-early-blue)
[![CI](https://github.com/NawaMan/MarkDownViewer/actions/workflows/ci.yml/badge.svg)](https://github.com/NawaMan/MarkDownViewer/actions/workflows/ci.yml)

## Features

- Serve any directory of `*.md` / `*.markdown` files over HTTP
- Sidebar file tree + GitHub-flavoured rendering ([Marked](https://github.com/markedjs/marked))
- Relative images and links work: they resolve against the file that wrote
  them, and links between Markdown files load in place (Back works)
- Resizable sidebar (width remembered), horizontal scroll, **Shift+wheel** for sideways scroll
- Copy button on every code block (falls back to a selection copy off localhost)
- Open a default file with `--md`
- **`--ask`**: prompt for the base folder or GitHub URL instead of requiring
  `--folder`/`DIR` up front — in the browser once it opens (a modal, prefilled
  with the default), or in the terminal under `--server-only`; the web UI
  keeps a "Change base…" button for the life of that instance so you can
  ask again later, same as `viewmd DIR`/the sidebar's "set as base" icons but
  by typing (see [Changing the base folder](#changing-the-base-folder-while-its-running))
- Opens your browser on start — `--server-only` when you would rather it did not
- **Change the base folder without restarting**: `viewmd DIR` retargets a
  running instance, and the sidebar has matching "set as base" icons —
  navigation always stays inside the directory viewmd was started with
- **Browse a GitHub repo directly**, no clone needed:
  `viewmd https://github.com/OWNER/REPO/tree/BRANCH/PATH` — pass
  `--github-token` (or `$GITHUB_TOKEN`) to raise the API's 60/hour
  unauthenticated rate limit or reach a private repo
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

Your browser opens on http://127.0.0.1:8765/ by itself. Add `--server-only` to
keep it closed — see [Opening a browser](#opening-a-browser).

### Download the File Directly

|  | Linux | macOS | Windows |
| --- | --- | --- | --- |
| **x86-64** | [viewmd&#8209;linux&#8209;amd64](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-linux-amd64) | [viewmd&#8209;darwin&#8209;amd64](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-darwin-amd64) | [viewmd&#8209;windows&#8209;amd64.exe](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-windows-amd64.exe) |
| **ARM64** | [viewmd&#8209;linux&#8209;arm64](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-linux-arm64) | [viewmd&#8209;darwin&#8209;arm64](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-darwin-arm64) | [viewmd&#8209;windows&#8209;arm64.exe](https://github.com/NawaMan/MarkDownViewer/releases/latest/download/viewmd-windows-arm64.exe) |

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
viewmd DIR [flags]
viewmd <command> [flags]

Commands:
  version            Print version and exit (same as --version)
  stop               Stop the background instance for --port (same as --stop)
  status             Report whether a background instance is running

Flags:
  --folder DIR       Root directory to scan, or a GitHub URL such as
                     https://github.com/OWNER/REPO/tree/BRANCH/PATH
                     (default "."); DIR alone (before any flags) is
                     shorthand for --folder DIR
  --ask               Prompt for the base folder/URL instead of using
                     --folder: in the browser once it opens, or right here
                     when --server-only (foreground only; cannot combine
                     with --daemon)
  --port N           Listen port (default 8765)
  --bind ADDR        Listen address (default 0.0.0.0)
  --md FILE          Open this Markdown file first (relative to --folder)
  --github-token TOK GitHub token for API access (raises the 60/hr
                     unauthenticated rate limit to 5,000/hr; also needed for
                     private repos); falls back to $GITHUB_TOKEN or $GH_TOKEN
  --expose [PORT]    After listen, run booth--expose <port> [PORT]
                     (no-op warning if booth--expose is not on PATH)
  --server-only      Do not open a browser (the default is to open one)
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
viewmd --ask                                        # prompt for the base folder/URL
viewmd --folder ./docs --md intro.md
viewmd --md README.md --port 9000
viewmd --md README.md --server-only                # serve only, no browser
viewmd --folder . --md README.md --expose          # host port = server port
viewmd --md README.md --expose 18765               # host 18765 → container port
```

## Browsing a GitHub repo

`--folder` (and the positional `DIR` shorthand) also accepts a GitHub URL. In
that case viewmd fetches Markdown and images straight from the GitHub API
instead of the local disk — nothing is cloned or written to disk:

```bash
viewmd https://github.com/NawaMan/CodingBooth/tree/main/docs
viewmd --folder https://github.com/OWNER/REPO              # default branch, repo root
viewmd --folder https://github.com/OWNER/REPO/tree/BRANCH/PATH
```

Everything else works the same way from there: the sidebar, relative
links/images, "set as base folder", and `viewmd DIR` retargeting — though a
GitHub-rooted instance can only retarget within the *same* repo and branch;
it cannot hop to a different repo, switch branches, or move between GitHub
and the local disk. A branch name containing `/` is not supported in the URL
(there is no reliable way to tell where the branch ends and the path begins).

GitHub's API rate-limits unauthenticated requests to 60/hour. viewmd does not
retry or paper over that — when the limit is hit, the error (including when
it resets) is shown wherever the request came from: the sidebar for a click,
the terminal for a bad `--folder` at startup. Give it a token to raise the
ceiling to 5,000/hour and to reach private repos:

```bash
viewmd --folder https://github.com/OWNER/REPO --github-token ghp_xxx
GITHUB_TOKEN=ghp_xxx viewmd https://github.com/OWNER/REPO   # or $GH_TOKEN
```

The token only needs `repo` (or, for a public repo, no scopes at all) read
access, and is only ever sent to `api.github.com`.

## Opening a browser

Starting the server opens your default browser on it, in the foreground and
under `--daemon` alike. The browser is launched once the port is bound, so it
cannot arrive before the server is ready, and the launcher is whatever the
platform provides — no dependency comes along for the ride:

| Platform | Launcher |
| --- | --- |
| macOS | `open` |
| Windows | `rundll32 url.dll,FileProtocolHandler` |
| Linux, BSD | `$BROWSER`, then `xdg-open`, `gio open`, `wslview`, `sensible-browser`, `x-www-browser`, `www-browser` |

The URL is not the listen address: `--bind` defaults to `0.0.0.0`, which is a
fine thing to listen on and not a thing a browser can visit — Windows refuses
it outright. An unspecified bind address becomes `127.0.0.1` (or `[::1]`) in
the URL, both in the browser and in the banner; any other address is used as
given. A bind that answers on every interface is still worth knowing about, so
the banner keeps a `listen:` line for it.

A headless server, a container and an SSH session have no browser to open, and
that is not an error: viewmd warns, prints the URL, and goes on serving. Pass
`--server-only` where that warning is just noise — CI, a container, a service
unit — and nothing is launched at all.

## Changing the base folder while it's running

viewmd has two folder concepts: the **starting folder** — whatever `--folder`
(or `viewmd DIR`) named at launch — and the **base folder**, whichever
directory the sidebar is currently rooted at. The base folder can move
around live; the starting folder never changes for the life of the process,
and every move is required to stay inside it.

In the sidebar, every folder gets a small "←" icon next to it: click it to
make that folder the new base. A `..` row appears at the top of the tree
whenever the base isn't already the starting folder, so you can walk back up
before descending into a different branch:

```text
Sidebar, base = /home/nawa (the starting folder):
  ..                          (not shown — already at the starting folder)
  docs/            [<-]
  manual/          [<-]

Click docs/'s icon -> base = /home/nawa/docs:
  ..               [<-]       (click = back to /home/nawa)
  api/             [<-]
  intro.md
```

`viewmd DIR` is the same idea from the command line: shorthand for
`viewmd --folder DIR`, but if an instance is already serving `--port` (found
via its pid file, the same one `--stop`/`--status` use), it retargets that
instance's base folder live instead of failing to bind a second time:

```bash
viewmd /home/nawa --daemon --port 8765   # start; /home/nawa is now the starting folder
viewmd docs --port 8765                  # same instance, base folder -> /home/nawa/docs
viewmd manual --port 8765                # base folder -> /home/nawa/manual (relative to the *starting* folder, not to docs)
viewmd / --port 8765                     # refused: outside the starting folder
```

A relative argument here is always resolved against the **starting** folder,
not wherever the base currently is — `manual` above means
`/home/nawa/manual` even though the base had moved to `docs`. An absolute
path is taken as given but still has to land inside the starting folder.
Either way, anything that resolves outside it is rejected with a 400 and the
instance keeps serving whatever it was serving before. (The sidebar icons
never rely on this: they always send the absolute path of whichever folder
was clicked.)

This works for both `--daemon` and plain foreground instances that were given
an explicit `--pidfile` (or started with `--daemon`); it dials `127.0.0.1` on
`--port` regardless of `--bind`, so it also needs matching `--port`/`--pidfile`
values, the same requirement `--stop` and `--status` already have.

`/api/folder` has no more (and no less) trust than every other endpoint here:
with no authentication, anyone who can reach the server can already read the
whole current tree under the starting folder, and retargeting only changes
which part of that tree is current — never which file types leave the process
(still just the [asset allowlist](#relative-images-and-links) below), and
never anything outside the folder viewmd was started with in the first place.

### `--ask` in the browser

`--ask` without `--server-only` skips the terminal prompt: the server binds
its port immediately, without a starting folder, and the browser it opens
shows a modal asking for one instead, prefilled with `--folder`'s value (`.`
by default) the same way the terminal prompt would be. Nothing under
`/api/tree`, `/api/file` or `/api/asset` works until that first pick lands —
they answer `503` — because until then there is no starting folder to bound
them to. That first pick is genuinely unbounded (it can be any local
directory or GitHub URL, not just a subdirectory of `--folder`'s default);
every pick after it goes through the same bounded retarget as `viewmd DIR`
and the sidebar's "set as base" icons.

Once a folder is picked, a **"Change base…"** button appears in the header
for the life of that instance (this is `cfg.ask` — true whether the terminal
or the browser ended up doing the asking) so you can ask again later, by
typing rather than by clicking through the tree.

## Relative images and links

A Markdown file writes its links relative to itself: `CodingBooth/README.md`
says `docs/logo.png` and means `CodingBooth/docs/logo.png`. The viewer is a
single page served at `/`, so the browser would otherwise resolve that against
the site root and miss by the file's own directory. viewmd rebases every local
target onto the directory of the file that wrote it, so pointing `--folder` at
a parent of the repo works the same as pointing it at the repo.

From there:

- **Images, video, audio, fonts and PDFs** are served from `/api/asset`.
- **Links to other Markdown files** load in place, so the sidebar and the
  address bar keep up and Back returns to the file the link was in. The `href`
  stays real, so middle-click and "open in new tab" still work.
- **Links off-site** open in a new tab; `#anchors` and `data:` URIs are left
  alone.

Only the file types a Markdown page can actually embed are served. A repo root
holds `.env`, `.git/config` and CI secrets next to its docs, and viewmd binds
`0.0.0.0` by default — so anything outside that list answers `403`, and a path
that climbs above `--folder` answers `400`.

## Daemon mode

`--daemon` re-execs `viewmd` detached from the terminal and returns as soon as
the port is bound, so a failure to start is still reported to your shell:

```bash
viewmd --folder ./docs --md intro.md --daemon
# viewmd v0.6.0 running in background (pid 12345)
#   url:      http://127.0.0.1:8765/
#   listen:   0.0.0.0:8765 (all interfaces)
#   pid file: /tmp/viewmd-8765.pid
#   log file: /tmp/viewmd-8765.log
#   stop it:  viewmd stop --port 8765
#   opening:  http://127.0.0.1:8765/

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
`binaries.yml`: it builds all six targets once and *runs* each one on a native
runner of its own platform — Linux x86-64 and ARM64, macOS Intel and Apple
Silicon, Windows x86-64 and ARM64 — serving a document over HTTP and reading it
back. The Linux x86-64 binary additionally has to start inside Alpine, which
has no glibc.

`Release` reuses that same workflow and publishes only what passed it.

To cut a release, bump `version.txt` and push a matching tag:

```bash
echo "0.2.0" > version.txt
git commit -am "Release 0.2.0"
git tag v0.2.0
git push origin main v0.2.0
```

`.github/workflows/release.yml` then tests, builds all six targets, and
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
