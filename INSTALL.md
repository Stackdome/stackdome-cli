# Install the Stackdome CLI

The installers download the latest GitHub release for the current platform and
verify its SHA-256 checksum before installing it.

## macOS and Linux

```sh
curl -fsSL https://raw.githubusercontent.com/Stackdome/stackdome-cli/main/install.sh | sh
```

For agents and CI, download first so a network failure cannot be hidden by
pipeline exit-status behavior:

```sh
installer_file=$(mktemp)
trap 'rm -f "$installer_file"' EXIT
curl -fsSL https://raw.githubusercontent.com/Stackdome/stackdome-cli/main/install.sh -o "$installer_file"
sh "$installer_file"
```

The installer supports Intel/AMD64 and ARM64. It writes to `/usr/local/bin`
when that directory is writable, otherwise it uses `$HOME/.local/bin`.

To install a specific version or directory:

```sh
curl -fsSL https://raw.githubusercontent.com/Stackdome/stackdome-cli/main/install.sh \
  | STACKDOME_VERSION=v0.0.1-alpha STACKDOME_INSTALL_DIR="$HOME/.local/bin" sh
```

## Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/Stackdome/stackdome-cli/main/install.ps1 | iex
```

For agents and CI, fetch the script before evaluating it so download errors are
terminal:

```powershell
$installer = Invoke-RestMethod -ErrorAction Stop https://raw.githubusercontent.com/Stackdome/stackdome-cli/main/install.ps1
& ([ScriptBlock]::Create([string]$installer))
```

The installer supports AMD64 and ARM64. By default it installs to
`%LOCALAPPDATA%\Programs\Stackdome\bin` and adds that directory to the user
`PATH`.

To install a specific version or directory:

```powershell
$env:STACKDOME_VERSION = 'v0.0.1-alpha'
$env:STACKDOME_INSTALL_DIR = "$env:LOCALAPPDATA\Programs\Stackdome\bin"
irm https://raw.githubusercontent.com/Stackdome/stackdome-cli/main/install.ps1 | iex
```

## Branded URLs

A branded CLI installation URL can proxy `install.sh` and `install.ps1` from
this repository. The self-hosted Stackdome installer remains a separate
artifact owned by the Hub repository and should not point at these CLI scripts.
