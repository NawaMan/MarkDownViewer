# AGENTS.md

Instructions for AI agents working in this repository.

## Commits

- **NEVER add `Co-Authored-By:` trailers.** No agent attribution of any kind
  belongs in a commit message — no `Co-Authored-By`, no "Generated with", no
  tool name, no emoji footer. This applies to commits, amends, and squashes.
- Subject line under 80 characters, imperative mood, no trailing period.
- Use the body to explain *why* when the diff does not make it obvious.

## Build and test

```bash
./build.sh              # ./viewmd for this machine
./build.sh --all        # also bin/viewmd-<os>-<arch> for all six targets
go test ./...           # full suite
gofmt -l cmd            # must print nothing
go vet ./...
```

Some tests build the binary and spawn real processes that bind real ports; they
skip under `go test -short`.

## Conventions

- Go 1.24+, standard library only — the module has no dependencies. Keep it
  that way unless there is a strong reason not to.
- OS-specific code goes in `_unix.go` / `_windows.go` files behind build tags.
  Anything added there must compile for every target in `build.sh --all`.
- `version.txt` is the single source of truth for the version. A release tag
  must match it (`v0.2.0` ↔ `0.2.0`) or the release workflow fails.
