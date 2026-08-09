#!/bin/sh

set -eu

repository=${STACKDOME_REPOSITORY:-Stackdome/stackdome-cli}
release_base_url=${STACKDOME_RELEASE_BASE_URL:-https://github.com/${repository}/releases/download}
api_base_url=${STACKDOME_API_BASE_URL:-https://api.github.com}
version=${STACKDOME_VERSION:-}
install_dir=${STACKDOME_INSTALL_DIR:-}
modify_path=true
tmp_dir=
staged_binary=

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Install the Stackdome CLI on macOS or Linux.

Options:
  --version <version>        Install a specific release (for example, v0.0.2-alpha)
  --install-dir <directory>  Install stackdome into this directory
  --no-modify-path           Do not persist the install directory in PATH
  -h, --help                 Show this help

Environment:
  STACKDOME_VERSION          Default release version
  STACKDOME_INSTALL_DIR      Default installation directory
EOF
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [ -n "$staged_binary" ] && [ -e "$staged_binary" ]; then
    rm -f "$staged_binary"
  fi
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

path_contains() {
  target_path=$1
  old_ifs=$IFS
  IFS=:
  glob_was_disabled=false
  case $- in
    *f*) glob_was_disabled=true ;;
    *) set -f ;;
  esac

  path_found=false
  for path_entry in ${PATH:-}; do
    if [ "$path_entry" = "$target_path" ]; then
      path_found=true
      break
    fi
  done

  [ "$glob_was_disabled" = true ] || set +f
  IFS=$old_ifs
  [ "$path_found" = true ]
}

quote_for_shell() {
  printf '%s' "$1" | sed "s/'/'\"'\"'/g"
}

append_line_once() {
  destination=$1
  line=$2

  if [ -f "$destination" ] && grep -Fqx "$line" "$destination"; then
    return 0
  fi
  printf '\n# Added by the Stackdome CLI installer\n%s\n' "$line" >>"$destination"
}

persist_path() {
  if [ -n "${GITHUB_PATH:-}" ]; then
    if ! grep -Fqx "$install_dir" "$GITHUB_PATH" 2>/dev/null; then
      printf '%s\n' "$install_dir" >>"$GITHUB_PATH" || return 1
    fi
    path_config_file=$GITHUB_PATH
    return 0
  fi

  [ -n "${HOME:-}" ] || return 1
  [ "$install_dir" = "$HOME/.local/bin" ] || return 1

  shell_path=${SHELL:-}
  shell_name=${shell_path##*/}
  case "$shell_name" in
    zsh)
      profile_dir=${ZDOTDIR:-$HOME}
      profile=$profile_dir/.zshrc
      ;;
    bash)
      profile_dir=$HOME
      if [ "$os" = darwin ]; then
        if [ -f "$HOME/.bash_profile" ]; then
          profile=$HOME/.bash_profile
        elif [ -f "$HOME/.bash_login" ]; then
          profile=$HOME/.bash_login
        elif [ -f "$HOME/.profile" ]; then
          profile=$HOME/.profile
        else
          profile=$HOME/.bash_profile
        fi
      else
        profile=$HOME/.bashrc
      fi
      ;;
    sh | dash | ksh)
      profile_dir=$HOME
      profile=$HOME/.profile
      ;;
    *) return 1 ;;
  esac

  mkdir -p "$profile_dir" || return 1
  append_line_once "$profile" 'export PATH="$HOME/.local/bin:$PATH"' || return 1
  path_config_file=$profile
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] && [ -n "$2" ] || fail "--version requires a value"
      version=$2
      shift 2
      ;;
    --install-dir)
      [ "$#" -ge 2 ] && [ -n "$2" ] || fail "--install-dir requires a value"
      install_dir=$2
      shift 2
      ;;
    --no-modify-path)
      modify_path=false
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    --)
      shift
      [ "$#" -eq 0 ] || fail "unexpected argument: $1"
      ;;
    -*) fail "unknown option: $1" ;;
    *) fail "unexpected argument: $1" ;;
  esac
done

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

if [ -n "$install_dir" ]; then
  :
elif path_contains /usr/local/bin && [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  install_dir=/usr/local/bin
elif [ -n "${HOME:-}" ] && path_contains "$HOME/.local/bin"; then
  install_dir=$HOME/.local/bin
elif [ -n "${HOME:-}" ] && path_contains "$HOME/bin" && [ -d "$HOME/bin" ] && [ -w "$HOME/bin" ]; then
  install_dir=$HOME/bin
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
staged_binary=$(mktemp "$install_dir/.stackdome.install.XXXXXX") || fail "could not stage stackdome in $install_dir"
install -m 0755 "$tmp_dir/stackdome" "$staged_binary" || fail "could not stage stackdome in $install_dir"
mv -f "$staged_binary" "$install_dir/stackdome" || fail "could not install stackdome to $install_dir"
staged_binary=

printf 'Installed Stackdome CLI %s to %s/stackdome\n' "$version" "$install_dir"
if ! path_contains "$install_dir"; then
  path_config_file=
  if [ "$modify_path" = true ] && persist_path; then
    printf 'Configured %s in PATH via %s.\n' "$install_dir" "$path_config_file" >&2
  elif [ "$modify_path" = true ]; then
    printf 'warning: %s is not on your PATH; add it before running stackdome\n' "$install_dir" >&2
  else
    printf 'PATH modification skipped by --no-modify-path.\n' >&2
  fi
  printf 'Run this command to use stackdome in the current shell:\n' >&2
  quoted_install_dir=$(quote_for_shell "$install_dir")
  printf '  export PATH='"'"'%s'"'"':"$PATH"\n' "$quoted_install_dir" >&2
fi
