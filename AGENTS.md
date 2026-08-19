# AGENTS.md

Instructions for AI agents working in this repository.

## Commits

- **NEVER add `Co-Authored-By:` trailers.** No agent attribution of any kind
  belongs in a commit message — no `Co-Authored-By`, no "Generated with", no
  tool name, no emoji footer. This applies to commits, amends, and squashes.
- Subject line under 80 characters, imperative mood, no trailing period.
- Use the body to explain *why* when the diff does not make it obvious.

## Running anything: use the booth

This project is developed inside a [CodingBooth](https://codingbooth.io) container.
The host has no guaranteed Go toolchain — it lives in the booth image, defined by
`.booth/Boothfile`.

| You are outside the booth | So… |
|---|---|
| **Editing files** | Do it on the host. The repo is bind-mounted into the booth at `/home/coder/code`, so edits are visible inside instantly — no copy, no sync, no restart. |
| **Running anything** (build, test, vet, gofmt, `go get`) | Never on the host. Always `./booth exec --run -- bash -lc '<cmd>'` |

`--run` starts the booth if it is not up; add `--keep-alive` so it stays up and
later commands are instant. `-d` detaches (and needs `--keep-alive`, or the booth
stops when exec returns and takes the process with it). Files created inside are
owned by your host user, so build artifacts are not root-owned.

### Gotchas (verified — do not rediscover these)

1. **`booth exec` does not go through a shell.** It execs argv directly, so
   `./booth exec -- 'go build && echo ok'` fails with `executable file not found
   in $PATH`. Wrap anything using `&&`, `|`, `cd`, globs or redirection in
   `bash -lc '...'`. (`./booth -- <cmd>` — run mode, no `exec` — *does* use the
   login shell. `exec` is the one you want, and it needs the explicit shell.)
2. **Exit codes are forwarded**, so `go vet` / `go test` failures fail the command
   normally and `&&` chaining works.
3. **`exec` accepts only its own flags** — `-run`, `-keep-alive`, `-d`/`-daemon`,
   `-it`, `-e`, `-envfile`, `-dir`, `-name`, `-port`, `-accept-existing`. Anything
   else makes it print usage instead of running your command. If output starts
   with `Usage of exec:`, that is what happened.
4. **`.booth/` is read-only inside the container.** Edit it from the host.
5. **`booth stop` + `exec --run` resumes the old container — it does not rebuild.**
   After changing `.booth/Boothfile` **or the port/publish settings in
   `.booth/config.toml`**, use `./booth remove -f` and then
   `./booth exec --run --keep-alive -- bash -lc 'echo ready'`. Confirm it took
   with `docker port MarkDownViewer`. Docker layers are cached, so this is
   usually seconds.

### Booth lifecycle

```bash
./booth list                                       # what is running (container name: MarkDownViewer)
./booth exec --run --keep-alive -- bash -lc '…'    # start if needed, keep it up
./booth stop                                       # stop, keep container (fast resume, same image)
./booth remove -f                                  # delete it (next --run rebuilds the image)
./booth shell                                      # interactive shell — for humans, not agents
```

## Build and test

```bash
./booth exec --run -- bash -lc 'cd /home/coder/code && ./build.sh'        # ./viewmd for this machine
./booth exec --run -- bash -lc 'cd /home/coder/code && ./build.sh --all'  # also bin/viewmd-<os>-<arch>, all six targets
./booth exec --run -- bash -lc 'cd /home/coder/code && go test ./...'     # full suite
./booth exec --run -- bash -lc 'cd /home/coder/code && gofmt -l cmd'      # must print nothing
./booth exec --run -- bash -lc 'cd /home/coder/code && go vet ./...'
```

Some tests build the binary and spawn real processes that bind real ports; they
skip under `go test -short`. Those ports are container-local, so the full suite
runs fine inside the booth.

## Serving viewmd so the host can see it

The booth publishes two ports (`.booth/config.toml`):

| Inside the booth | On the host | What it is |
|---|---|---|
| `10000` | *booth port* (`127.0.0.1`) | front door — leave it alone |
| `987` | *booth port* `+987` (`0.0.0.0`) | **serve viewmd here** |

The booth port is `${NEXT:-33000}`, so it is **allocated at container-create time
and is not fixed** — never hardcode it. Look it up with `./booth list` (PORT
column) or `docker port MarkDownViewer`. At the time of writing it is `33000`,
which puts viewmd at `http://localhost:33987`.

```bash
./booth exec --run --keep-alive -- bash -lc \
  'cd /home/coder/code && ./viewmd --folder . --md README.md --port 987 --bind 0.0.0.0 --server-only --daemon'
curl -s http://localhost:33987/                                       # from the host (33000 + 987)
./booth exec -- bash -lc 'tail /tmp/viewmd-987.log'                   # logs
./booth exec -- bash -lc 'cd /home/coder/code && ./viewmd stop --port 987'
```

`987` is a privileged port, but the container is configured so the unprivileged
`coder` user can bind it — no `sudo` needed. `--server-only` is what keeps the
booth from hunting for a browser it does not have; without it viewmd prints a
warning and serves anyway.

`--expose` (which shells out to `booth--expose`) registers a tunnel but did **not**
produce a host-side listener under this `terminal` variant — `booth expose list`
showed the tunnel as live while nothing was bound on the host. It also leaves
`.booth/.tmp/tcp-tunnels/<port>` behind, which makes the next attempt fail with
`tunnel for container port N already exists`; delete that file from the host to
reset. Prefer the front-door port above.

## Conventions

- Go 1.24+, standard library only — the module has no dependencies. Keep it
  that way unless there is a strong reason not to.
- OS-specific code goes in `_unix.go` / `_windows.go` files behind build tags.
  Anything added there must compile for every target in `build.sh --all`.
- `version.txt` is the single source of truth for the version. A release tag
  must match it (`v0.2.0` ↔ `0.2.0`) or the release workflow fails.
