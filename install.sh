#!/bin/sh
# armup installer (Linux / macOS).
#
#   curl -sSL https://raw.githubusercontent.com/mihnen/armup/master/install.sh | sh
#
# Picks the latest release, verifies SHA-256, drops `armup` into
# $ARMUP_INSTALL_DIR (default: $HOME/.local/bin). Then run `armup init`.

set -eu

OWNER=mihnen
REPO=armup
INSTALL_DIR="${ARMUP_INSTALL_DIR:-$HOME/.local/bin}"

err() { printf '%s\n' "$*" >&2; }

# Detect platform.
case "$(uname -s)" in
  Linux)  os=linux  ;;
  Darwin) os=darwin ;;
  *) err "armup install.sh supports Linux and macOS only (got $(uname -s)). For Windows use install.ps1."; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64)  arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) err "unsupported architecture: $(uname -m)"; exit 1 ;;
esac

# Pick a SHA-256 tool: GNU coreutils on Linux, BSD shasum on macOS.
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  err "no sha256 tool found (need sha256sum or shasum)"; exit 1
fi

# Find the latest release tag.
echo "Querying latest release..."
tag=$(curl -fsSL "https://api.github.com/repos/$OWNER/$REPO/releases/latest" \
  | sed -nE 's/.*"tag_name": *"([^"]+)".*/\1/p' | head -n1)
if [ -z "$tag" ]; then
  err "failed to determine latest release"; exit 1
fi
echo "Latest release: $tag"

file="armup-${tag}-${os}-${arch}.tar.gz"
url="https://github.com/$OWNER/$REPO/releases/download/$tag/$file"
sums_url="https://github.com/$OWNER/$REPO/releases/download/$tag/SHA256SUMS"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $url"
curl -fsSL -o "$tmp/$file" "$url"

echo "Verifying checksum"
curl -fsSL -o "$tmp/SHA256SUMS" "$sums_url"
expected=$(awk -v f="$file" '$2 == f || $2 == "*"f { print $1; exit }' "$tmp/SHA256SUMS")
if [ -z "$expected" ]; then
  err "could not find $file in SHA256SUMS"; exit 1
fi
actual=$(sha256 "$tmp/$file")
if [ "$expected" != "$actual" ]; then
  err "checksum mismatch: expected $expected, got $actual"; exit 1
fi

mkdir -p "$INSTALL_DIR"
tar -xzf "$tmp/$file" -C "$tmp"
mv "$tmp/armup-${tag}-${os}-${arch}/armup" "$INSTALL_DIR/armup"
chmod +x "$INSTALL_DIR/armup"

echo
echo "Installed armup $tag to $INSTALL_DIR/armup"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo
    echo "WARNING: $INSTALL_DIR is not on your PATH."
    echo "Add this to your shell profile:"
    echo "    export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac
echo
echo "Next: run 'armup init' to create the toolchain data directory and"
echo "      add its bin/ to PATH, then open a fresh shell."
