#!/bin/sh

set -eu

repository=${STACKDOME_REPOSITORY:-Stackdome/stackdome-cli}
release_base_url=${STACKDOME_RELEASE_BASE_URL:-https://github.com/${repository}/releases/download}
api_base_url=${STACKDOME_API_BASE_URL:-https://api.github.com}
version=${STACKDOME_VERSION:-}
tmp_dir=

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [ -n "$tmp_dir" ] && [ -d "$tmp_dir" ]; then
    rm -rf "$tmp_dir"
  fi
}

download() {
  source_url=$1
  destination=$2

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$source_url" -o "$destination"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$source_url" -O "$destination"
  else
    fail "curl or wget is required to download Stackdome CLI"
  fi
}

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "unsupported operating system: $(uname -s) (supported: macOS and Linux)" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported architecture: $(uname -m) (supported: amd64 and arm64)" ;;
esac

if [ -n "${STACKDOME_INSTALL_DIR:-}" ]; then
  install_dir=$STACKDOME_INSTALL_DIR
elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  install_dir=/usr/local/bin
else
  [ -n "${HOME:-}" ] || fail "HOME is not set; set STACKDOME_INSTALL_DIR"
  install_dir=$HOME/.local/bin
fi

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/stackdome-install.XXXXXX") || fail "could not create a temporary directory"
trap cleanup 0
trap 'exit 1' HUP INT TERM

if [ -z "$version" ]; then
  release_metadata=$tmp_dir/latest-release.json
  download "${api_base_url}/repos/${repository}/releases/latest" "$release_metadata" || fail "could not discover the latest Stackdome CLI release; check network access or set STACKDOME_VERSION explicitly"
  version=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$release_metadata" | sed -n '1p')
  [ -n "$version" ] || fail "latest release response did not contain a tag_name; set STACKDOME_VERSION explicitly"
fi

case "$version" in
  *[!A-Za-z0-9._-]*) fail "invalid release version: $version" ;;
esac

asset="stackdome_${version}_${os}_${arch}.tar.gz"
asset_url="${release_base_url}/${version}/${asset}"
checksums_url="${release_base_url}/${version}/checksums.txt"

archive_path=$tmp_dir/$asset
checksums_path=$tmp_dir/checksums.txt

printf 'Downloading Stackdome CLI %s (%s/%s)...\n' "$version" "$os" "$arch"
download "$asset_url" "$archive_path" || fail "could not download release asset $asset"
download "$checksums_url" "$checksums_path" || fail "could not download checksums.txt for $version"

checksum=$(awk -v asset="$asset" '$2 == asset || $2 == "*" asset { print $1; exit }' "$checksums_path")
[ -n "$checksum" ] || fail "checksums.txt does not contain an exact entry for $asset"

if command -v sha256sum >/dev/null 2>&1; then
  actual_checksum=$(sha256sum "$archive_path" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
  actual_checksum=$(shasum -a 256 "$archive_path" | awk '{ print $1 }')
else
  fail "sha256sum or shasum is required to verify the download"
fi

[ "$actual_checksum" = "$checksum" ] || fail "checksum verification failed for $asset"
printf 'Verified checksum for %s.\n' "$asset"

tar -xzf "$archive_path" -C "$tmp_dir" stackdome || fail "could not extract stackdome from $asset"
mkdir -p "$install_dir" || fail "could not create install directory: $install_dir"
install -m 0755 "$tmp_dir/stackdome" "$install_dir/stackdome" || fail "could not install stackdome to $install_dir"

printf 'Installed Stackdome CLI to %s/stackdome\n' "$install_dir"
case ":${PATH:-}:" in
  *":$install_dir:"*) ;;
  *) printf 'warning: %s is not on your PATH; add it before running stackdome\n' "$install_dir" >&2 ;;
esac
