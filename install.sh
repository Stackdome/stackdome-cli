#!/bin/sh
# Installs the latest stackdome CLI from GitHub releases.
# Usage: curl -fsSL https://get.stackdome.com/cli | sh
set -eu

REPO="Stackdome/stackdome-cli"

os=$(uname -s | tr '[:upper:]' '[:lower:]')   # darwin | linux
arch=$(uname -m)
case "$arch" in
  x86_64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) echo "error: unsupported architecture: $arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin | linux) ;;
  *) echo "error: unsupported OS: $os" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | cut -d '"' -f 4)
[ -n "$tag" ] || { echo "error: could not resolve latest release" >&2; exit 1; }
version=${tag#v}

asset="stackdome_${version}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${tag}/${asset}"

if [ -w /usr/local/bin ]; then
  bindir="/usr/local/bin"
else
  bindir="${HOME}/.local/bin"
  mkdir -p "$bindir"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Downloading stackdome ${tag} (${os}/${arch})..."
curl -fsSL "$url" -o "${tmp}/${asset}"
tar -xzf "${tmp}/${asset}" -C "$tmp" stackdome
install -m 0755 "${tmp}/stackdome" "${bindir}/stackdome"

echo "Installed to ${bindir}/stackdome"
case ":$PATH:" in
  *":${bindir}:"*) "${bindir}/stackdome" version ;;
  *) echo "warning: ${bindir} is not on your PATH" >&2 ;;
esac
