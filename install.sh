#!/bin/sh
# enthea — universal installer.
# Works on any Unix (macOS, Debian/Ubuntu, Alpine, Fedora, …) and Windows
# under git-bash/WSL — it ignores brew/apt/apk entirely and drops the static
# binary for your OS/arch into ~/.local/bin.
#
#   curl -fsSL https://raw.githubusercontent.com/8b-is/enthea/main/install.sh | sh
set -eu

VERSION="${ENTHEA_VERSION:-latest}"
DEST="${ENTHEA_DEST:-$HOME/.local/bin}"
BASE="https://github.com/8b-is/enthea/releases/${VERSION}/download"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  darwin) os=darwin ;;
  linux)  os=linux ;;
  msys*|mingw*|cygwin*) os=windows ;;
  *) echo "enthea: unsupported os: $os" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  i386|i686) arch=386 ;;
  *) echo "enthea: unsupported arch: $arch" >&2; exit 1 ;;
esac

file="enthea-${os}-${arch}"
[ "$os" = windows ] && file="${file}.exe"
url="$BASE/$file"

echo "enthea ${VERSION} (${os}/${arch}) -> ${DEST}"
mkdir -p "$DEST"
curl -fsSL "$url" -o "$DEST/enthea"
chmod +x "$DEST/enthea"

echo
echo "enthea installed to $DEST/enthea"
case ":$PATH:" in
  *":$DEST:"*) ;;
  *) echo "add it to your PATH:  export PATH=\"$DEST:\$PATH\"" ;;
esac
echo "then: enthea setup opencode   # wire MCP + personas into your client"
