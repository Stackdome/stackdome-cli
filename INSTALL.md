# Install the Stackdome CLI

The installer downloads the latest GitHub release for the current platform and
verifies its SHA-256 checksum before installing it.

## macOS and Linux

```sh
curl -fsSL https://get.stackdome.com/cli.sh | sh
```

For agents and CI, download first so a network failure cannot be hidden by
pipeline exit-status behavior:

```sh
installer_file=$(mktemp)
trap 'rm -f "$installer_file"' EXIT
curl -fsSL https://get.stackdome.com/cli.sh -o "$installer_file"
sh "$installer_file"
```

The installer supports Intel/AMD64 and ARM64. It writes to `/usr/local/bin`
when that directory is writable, otherwise it uses `$HOME/.local/bin`.

To install a specific version or directory:

```sh
installer_file=$(mktemp)
trap 'rm -f "$installer_file"' EXIT
curl -fsSL https://get.stackdome.com/cli.sh -o "$installer_file"
sh "$installer_file" \
  --version v0.0.2-alpha \
  --install-dir "$HOME/.local/bin"
```

`STACKDOME_VERSION` and `STACKDOME_INSTALL_DIR` provide the same settings for
automation. Explicit flags take precedence over environment variables.
