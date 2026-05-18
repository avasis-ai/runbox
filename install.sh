#!/usr/bin/env bash
set -euo pipefail

REPO="avasis-ai/runbox"
BINARY="runbox"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${1:-latest}"

msg() { echo "  $1"; }
err() { echo "  error: $1" >&2; exit 1; }

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux" ;;
  *)      err "Unsupported OS: $OS" ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)            err "Unsupported architecture: $ARCH" ;;
esac

msg "Runbox Installer"
echo ""

if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/' || echo "")
  if [ -z "$VERSION" ]; then
    err "Could not determine latest version. Check https://github.com/$REPO/releases"
  fi
fi

FILENAME="${BINARY}_${VERSION#\v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/${VERSION}/${FILENAME}"

msg "Version:  $VERSION"
msg "OS/Arch:  $OS/$ARCH"
msg "Download: $URL"
echo ""

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

msg "Downloading..."
curl -fsSL "$URL" -o "$TMPDIR/$FILENAME" || err "Download failed. Binary may not exist for $OS/$ARCH yet."

msg "Extracting..."
tar -xzf "$TMPDIR/$FILENAME" -C "$TMPDIR" "$BINARY" 2>/dev/null || err "Extraction failed."

mkdir -p "$INSTALL_DIR"
chmod +x "$TMPDIR/$BINARY"
mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"

echo ""
msg "Installed: $INSTALL_DIR/$BINARY"

# PATH check
case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    msg "PATH:     OK ($INSTALL_DIR is in PATH)"
    ;;
  *)
    msg "PATH:     $INSTALL_DIR is NOT in PATH"
    msg ""
    msg "Add this to your shell profile (~/.zshrc or ~/.bashrc):"
    msg "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    ;;
esac

echo ""
INSTALLED=$("$INSTALL_DIR/$BINARY" version 2>/dev/null || echo "installed")
msg "$INSTALLED"
msg ""
msg "Quick start:"
msg "  runbox init mini --host mini --user you --workdir ~/project"
msg "  runbox doctor mini"
msg "  runbox fix mini --all"
msg "  runbox exec mini \"echo hello from \$(hostname)\""
