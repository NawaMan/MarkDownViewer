#!/usr/bin/env bash
# Build viewmd for the current platform (and optionally cross-compile).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

# This project builds inside a CodingBooth container (see AGENTS.md) — the host
# has no guaranteed Go toolchain. If we're not already inside one, hop in via
# ./booth instead of failing on a missing `go`. GitHub Actions is the one
# exception: its own workflow already pins a Go version via actions/setup-go
# and go.mod before calling this script, which is the same reproducibility
# guarantee the booth exists to provide on an arbitrary host — hopping in
# there would only add a Docker/CodingBooth-CLI dependency the runner may not
# have primed, for no benefit.
if [[ ! -d /opt/codingbooth && -z "${CB_CONTAINER_NAME:-}" && -z "${GITHUB_ACTIONS:-}" ]]; then
  if [[ ! -x ./booth ]]; then
    echo "build.sh: this project builds inside a CodingBooth container, but ./booth is missing or not executable here." >&2
    echo "  See AGENTS.md for details, or get CodingBooth: https://codingbooth.io" >&2
    exit 1
  fi
  if ! command -v docker >/dev/null 2>&1; then
    echo "build.sh: this project builds inside a CodingBooth container, which needs Docker, but 'docker' was not found on PATH." >&2
    echo "  Install Docker (or start it), then re-run ./build.sh." >&2
    exit 1
  fi
  if ! docker info >/dev/null 2>&1; then
    echo "build.sh: Docker is installed but not running (or not reachable). Start Docker and re-run ./build.sh." >&2
    exit 1
  fi
  args=""
  for a in "$@"; do
    args="$args $(printf '%q' "$a")"
  done
  exec ./booth exec --run -- bash -lc "cd /home/coder/code && ./build.sh$args"
fi

VERSION="$(tr -d ' \t\n\r' < version.txt 2>/dev/null || echo dev)"
LDFLAGS="-X main.version=${VERSION}"
APP=viewmd
PKG=./cmd/viewmd

# Pure Go, no cgo. Cross-compiled targets get this for free, but a native
# linux/amd64 build would otherwise link against the host glibc and refuse to
# start on musl (Alpine) or a distroless-static image.
export CGO_ENABLED=0

mkdir -p bin

echo "Building ${APP} v${VERSION}"
LOCAL="./${APP}"
case "$(uname -s)" in
  MINGW*|CYGWIN*|MSYS*) LOCAL="./${APP}.exe" ;;
esac

go build -ldflags "$LDFLAGS" -o "$LOCAL" "$PKG"
echo "  -> $LOCAL ($(du -h "$LOCAL" | cut -f1))"

if [[ "${1:-}" == "--all" ]]; then
  for pair in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
    GOOS="${pair%/*}"
    GOARCH="${pair#*/}"
    out="bin/${APP}-${GOOS}-${GOARCH}"
    [[ "$GOOS" == windows ]] && out="${out}.exe"
    echo -n "  ${GOOS}/${GOARCH} ... "
    GOOS="$GOOS" GOARCH="$GOARCH" go build -ldflags "$LDFLAGS" -o "$out" "$PKG"
    echo "ok ($(du -h "$out" | cut -f1))"
  done
fi

echo "Done."
