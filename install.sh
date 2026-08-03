#!/bin/sh
# Install viewmd as "viewmd" on this machine (Linux and macOS).
#
#   curl -fsSL https://github.com/NawaMan/MarkDownViewer/releases/latest/download/install.sh | sh
#
# Environment:
#   VIEWMD_VERSION      Tag to install (e.g. v0.3.0). Defaults to the release
#                       this copy of the script shipped with.
#   VIEWMD_INSTALL_DIR  Where to put the binary. Defaults to /usr/local/bin when
#                       writable, otherwise ~/.local/bin.
set -eu

REPO=NawaMan/MarkDownViewer

# The release workflow rewrites this line to the tag being published, so a
# script pulled from a pinned release installs that same release. A copy from a
# git checkout still says "dev" and means "whatever is newest".
VERSION_DEFAULT=dev

die() { echo "install.sh: $*" >&2; exit 1; }

version="${VIEWMD_VERSION:-$VERSION_DEFAULT}"
if [ "$version" = dev ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$version"
fi

os="$(uname -s)"
case "$os" in
  Linux)  os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported OS \"$os\" — on Windows run install.ps1 instead" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die "unsupported architecture \"$arch\"" ;;
esac

asset="viewmd-${os}-${arch}"

fetch() { # url dest
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2" || die "download failed: $1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1" || die "download failed: $1"
  else
    die "need curl or wget"
  fi
}

sha256() { # file
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

dir="${VIEWMD_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -w /usr/local/bin ]; then dir=/usr/local/bin; else dir="$HOME/.local/bin"; fi
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Downloading $asset ($version)"
fetch "$base/$asset" "$tmp/viewmd"
fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS"

want="$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/SHA256SUMS")"
[ -n "$want" ] || die "SHA256SUMS has no entry for $asset"
got="$(sha256 "$tmp/viewmd")"
if [ -z "$got" ]; then
  echo "install.sh: no sha256sum/shasum found — skipping checksum verification" >&2
elif [ "$got" != "$want" ]; then
  die "checksum mismatch for $asset (expected $want, got $got)"
fi

chmod +x "$tmp/viewmd"
mkdir -p "$dir" || die "cannot create $dir"
mv "$tmp/viewmd" "$dir/viewmd" || die "cannot write to $dir (set VIEWMD_INSTALL_DIR or re-run with sudo)"

echo "Installed viewmd $("$dir/viewmd" version) to $dir/viewmd"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "Note: $dir is not on your PATH — add it, or run $dir/viewmd directly" ;;
esac
