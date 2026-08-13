# Stackdome CLI

**Breeze through deploys.**

Deploy, debug, and operate applications with the Stackdome CLI. Start on Stackdome Cloud or connect
a self-hosted installation — your Stackfile and deployment commands stay the same.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8.svg)](go.mod)
[![Release](https://img.shields.io/github/v/release/Stackdome/stackdome-cli)](https://github.com/Stackdome/stackdome-cli/releases/latest)

`stackdome` is the execution interface for Stackdome. You describe your application once in a
`stackfile.yaml`, then deploy it, watch it converge, stream its logs, and manage its secrets,
volumes, and Postgres addons — all from the terminal, all scriptable.

> **Alpha:** Stackdome Cloud is ephemeral and capacity-limited during alpha, and each organization
> can connect one cluster. See [alpha limitations](https://docs.stackdome.com/reference/alpha-limitations).

## Install

```bash
curl -fsSL https://get.stackdome.com/cli.sh | sh
```

Installs the latest release to `/usr/local/bin` if writable, otherwise `~/.local/bin`, verifying
the published checksum. Supports macOS and Linux on amd64 and arm64. Set `STACKDOME_VERSION` to pin
a release or `STACKDOME_INSTALL_DIR` to choose the directory:

```bash
curl -fsSL https://get.stackdome.com/cli.sh | STACKDOME_VERSION=v0.0.5-alpha sh
```

Prebuilt archives are also on the
[releases page](https://github.com/Stackdome/stackdome-cli/releases/latest).

From source (Go 1.25+):

```bash
make build     # -> bin/stackdome
```

## Quickstart

Create an API token at `<your-stackdome-host>/settings/api-tokens`, then run from your repository
root:

```bash
stackdome login --url https://<your-stackdome-host> --token <api-token>
stackdome ctx -o json
stackdome init
stackdome validate
stackdome deploy --wait -o json
stackdome status -o json
```

`stackdome ctx` shows the active server, organization, project, selected stack, and authentication
source without revealing credentials — run it before deploying to confirm you are pointed at the
right place.

`stackdome init` converts an existing `docker-compose.yaml` when it finds one, and otherwise writes
a starter `stackfile.yaml`. Edit it to match your repository, re-run `stackdome validate`, then
deploy.

`stackdome deploy --wait` blocks until the release reaches a terminal state and exits non-zero
unless it reaches `Released`. To get a public URL without opening a browser:

```bash
stackdome open -o json
```

## The Stackfile

```yaml
name: my-app

resources:
  web:
    image: nginx:latest
    ports:
      - name: http
        port: 8080
        public: true
        subdomain: web
    env:
      APP_ENV: "production"
      PUBLIC_URL: "{{ self.public_url }}"
      DB_URL: "postgres://{{ db.host }}:{{ db.port }}/mydb"
    depends_on: [db]

  db:
    image: postgres:16
    ports:
      - name: postgres
        port: 5432
    env:
      POSTGRES_DB: mydb
    volumes:
      - name: db-data
        path: /var/lib/postgresql/data

volumes:
  db-data:
    size: 5Gi
```

Resources can also be built from git (`build.repo`) instead of an image, and can reference secrets
and addons. Print the full schema with `stackdome get stackfile-schema`, or read the
[Stackfile reference](https://docs.stackdome.com/reference/stackfile).

## Commands

Commands are grouped by what they do. Read verbs (`get`, `list`, `describe`) and change verbs
(`create`, `delete`, `update`) take a resource type as their subcommand — for example
`stackdome get stacks`, `stackdome describe release <id>`, `stackdome create secret <name>`.

| Command | Description |
| --- | --- |
| **Read** | |
| `get` | Get resources — `stack`, `secret`, `release`, `build`, `config`, `postgres-addon`, `stackfile-schema`, … |
| `list` | List resource collections — `stacks`, `secrets`, `releases`, `builds`, `volumes`, `tokens`, … |
| `describe` | Describe one resource — `stack`, `release`, `build`, `secret`, `postgres-addon` |
| **Change** | |
| `create` | Create a `secret`, `volume`, `token`, `postgres-addon`, or `release` |
| `update` | Update a `secret` |
| `delete` | Delete a `stack`, `secret`, `volume`, `token`, or `postgres-addon` |
| **Deploy and release** | |
| `deploy` | Deploy a stack from a stackfile or JSON |
| `apply` | Save a stack definition without releasing it |
| `rollback` | Roll back a release |
| `cancel` | Cancel a pending release |
| **Observe and operate** | |
| `status` | Show stack and resource status |
| `logs` | Read or follow runtime logs, or `logs build <id>` for build logs |
| `restart` | Restart a stack resource |
| `open` | Open a public resource URL |
| `backup` | Back up a `postgres-addon` |
| **Auth and context** | |
| `login` / `logout` | Authenticate with an API token; clear stored credentials |
| `signup` | Show web signup and CLI setup instructions |
| `whoami` | Show the active identity and scope |
| `ctx` | Show the active context |
| `use` | Select the active `context` (server URL) or `stack` |
| **Local tooling** | |
| `init` | Scaffold a Stackfile, or convert a Compose file |
| `validate` | Validate a Stackfile |
| `export` | Export resources as a `stackfile` |
| `api` | Send an authenticated raw API request |
| `doctor` | Diagnose CLI connectivity and context |
| `version` | Print the CLI version |
| `completion` | Generate a shell completion script |

Run `stackdome <command> --help` for flags, or see the
[CLI reference](https://docs.stackdome.com/reference/cli).

## For coding agents and CI

Every command runs non-interactively. Pass `--yes` to skip confirmations and `-o json|yaml` for
machine-readable output. In `json`/`yaml` mode **stdout carries only the result object** — all
prose, prompts, and progress go to stderr.

Authenticate without a config file:

```bash
export STACKDOME_URL=https://<your-stackdome-host>
export STACKDOME_TOKEN=<api-token>
stackdome whoami -o json
```

Tokens supplied through the environment are never written to disk. A token scoped too narrowly to
look up projects can name its scope directly with `STACKDOME_ORG` and `STACKDOME_PROJECT`. When
something looks wrong, `stackdome doctor` checks connectivity and context in one call.

Exit codes:

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | General error |
| `2` | Authentication / authorization failure |
| `3` | Not found |
| `4` | Invalid input or usage |
| `5` | Conflict (already exists, or state does not allow the operation) |
| `130` | Canceled (interrupted, or a confirmation was declined) |

Agents should install the maintained deploy skill with `npx skills add stackdome/skills` and follow
the canonical workflow in [Deploy with a coding agent](https://docs.stackdome.com/guides/ai-agents).
Never request a user's account password — Stackdome uses API-token authentication.

## Configuration

Credentials and context live in `~/.stackdome/config.json` (created `0600`, in a `0700` directory).
Point `STACKDOME_CONFIG` elsewhere to override the path.

| Variable | Purpose |
| --- | --- |
| `STACKDOME_URL` | Server URL |
| `STACKDOME_TOKEN` | API token (ephemeral, never persisted) |
| `STACKDOME_ORG` | Organization, for narrowly scoped tokens |
| `STACKDOME_PROJECT` | Project, for narrowly scoped tokens |
| `STACKDOME_CONFIG` | Config file path |

## Development

```bash
make build      # build bin/stackdome with version metadata
make clean      # remove bin/
go test ./...   # run the test suite
```

Layout: `cmd/stackdome/` holds one file per command, with the verb/resource tree wired up in
`command_tree.go`; `internal/` holds the API client, config, output formatting, error mapping, and
stackfile handling; `config/stackfile_schema.yaml` is the Stackfile schema.

## Links

- [Documentation](https://docs.stackdome.com)
- [Stackdome](https://stackdome.com)
- [Issues](https://github.com/Stackdome/stackdome-cli/issues)

## License

MIT — see [LICENSE](LICENSE).
