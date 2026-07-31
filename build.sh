#!/usr/bin/env bash
# Build viewmd for the current platform (and optionally cross-compile).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

VERSION="$(tr -d ' \t\n\r' < version.txt 2>/dev/null || echo dev)"
LDFLAGS="-X main.version=${VERSION}"
APP=viewmd
PKG=./cmd/viewmd

mkdir -p bin

echo "Building ${APP} v${VERSION}"
LOCAL="./${APP}"
case "$(uname -s)" in
  MINGW*|CYGWIN*|MSYS*) LOCAL="./${APP}.exe" ;;
esac

go build -ldflags "$LDFLAGS" -o "$LOCAL" "$PKG"
echo "  -> $LOCAL ($(du -h "$LOCAL" | cut -f1))"

if [[ "${1:-}" == "--all" ]]; then
  for pair in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
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
