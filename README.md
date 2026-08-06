# mkdb

A CLI tool to easily create and manage disposable local Docker database containers for development.

## Features

- **Multiple database types**: PostgreSQL, MySQL, and Redis
- **Positional or interactive**: `mkdb create postgres --name app`, or run with no flags and answer prompts
- **Script-friendly**: connection strings go to stdout, status messages to stderr, and fully-flagged commands never prompt
- **Agent-ready**: `--json` output, stable exit codes, a readiness wait on `create`, and a bundled [agent skill](skills/mkdb/SKILL.md)
- **Schema seeding**: load a SQL script at create time with `--init`
- **Automatic expiry**: containers expire after 2 hours by default (configurable) and are surfaced for cleanup
- **Encrypted credentials**: passwords stored with AES-256-GCM
- **Volume management**: no volume, an mkdb-managed named volume, or a bind mount to any path
- **User management**: add and remove additional database users
- **TTL management**: extend a container's lifetime to keep it around

## Prerequisites

- Go 1.25 or later (to build from source)
- Docker installed and running

## Installation

### Pre-built binaries (recommended)

Download the latest release for your platform from the [Releases page](https://github.com/pbzona/mkdb/releases):

- **Linux (amd64)**: `mkdb-linux-amd64`
- **Linux (arm64)**: `mkdb-linux-arm64`
- **macOS (Intel)**: `mkdb-darwin-amd64`
- **macOS (Apple Silicon)**: `mkdb-darwin-arm64`

```bash
# Example: install on macOS (Apple Silicon)
curl -L https://github.com/pbzona/mkdb/releases/latest/download/mkdb-darwin-arm64 -o mkdb
chmod +x mkdb
sudo mv mkdb /usr/local/bin/
```

### From source

```bash
git clone https://github.com/pbzona/mkdb.git
cd mkdb
go build -ldflags="-s -w" -o mkdb .
sudo mv mkdb /usr/local/bin/
```

### Using go install

```bash
go install github.com/pbzona/mkdb@latest
```

## Quick start

```bash
# Create a database (prompts for anything you don't pass as a flag)
mkdb create postgres --name app

# List your databases
mkdb ls

# Inspect one (add --ping to test connectivity)
mkdb info app --ping

# Grab the connection string
mkdb creds show app

# Stop it (data is preserved), then start it again
mkdb stop app
mkdb start app

# Delete it and its volume
mkdb rm app --yes
```

Most commands accept the database name as a positional argument or via `--name`.
Omit both to pick from an interactive list. When there is no terminal (CI, pipes),
commands never prompt: pass the required flags or you'll get a clear error.

## Commands

### `mkdb create [type]`

Create and start a new database container. `type` is one of `postgres`, `mysql`, or `redis`
(aliases accepted, see below) and may be given as an argument or chosen interactively.

**Flags:**
- `--name` — database name
- `--version` — image version (defaults: postgres=18, redis=8, mysql=latest)
- `--port` — host port to bind (defaults to the engine's port, auto-advancing if taken)
- `--volume` — `none`, `named`, or a host path for a bind mount
- `--ttl` — time to live in hours (default: 2)
- `--repeat` — reuse the settings from the last database created
- `--no-auth` — create without authentication
- `--init` — path to a SQL script applied once the database is ready (postgres/mysql)
- `--no-wait` — return immediately instead of waiting for the database to accept connections
- `--wait-timeout` — seconds to wait for readiness (default: 30)

By default `create` blocks until the database actually accepts connections, so the printed
connection string is immediately usable. If readiness isn't reached within `--wait-timeout`,
the command still prints the connection details but exits with code 4.

**Authentication** is enabled by default. A random 20-character password is generated and
shown as part of the connection string. Pass `--no-auth` to create an open database
(Postgres trust mode, MySQL empty root password, Redis without `requirepass`). When neither
`--no-auth` is given and you're in a terminal, mkdb asks whether to enable authentication
(default: yes).

**Examples:**
```bash
# Fully non-interactive
mkdb create postgres --name app --version 16 --port 5433

# Prompts only for what's missing
mkdb create mysql

# No persistent storage
mkdb create redis --name cache --volume none

# mkdb-managed named volume (~/.local/share/mkdb/volumes/<name>)
mkdb create postgres --name app --volume named

# Bind mount a host path
mkdb create redis --name cache --volume /data/redis

# Open database, longer lifetime
mkdb create postgres --name public --no-auth --ttl 48

# Reuse the previous settings
mkdb create --repeat

# Create and load a schema, then tear it down when done
mkdb create postgres --name app --init ./schema.sql
```

### `mkdb ls` (alias `list`)

List database containers and their configured major versions.

**Flags:**
- `--type` — filter by type (`postgres`, `mysql`, `redis`; aliases accepted)
- `--status` — filter by status (`running`/`up`, `stopped`/`down`, `expired`, `removed`)
- `--all`, `-a` — also show removed databases whose volume directories remain on disk

```bash
mkdb ls
mkdb ls --type postgres
mkdb ls --status running
mkdb ls -a
```

### `mkdb info [name]` (alias `stat`)

Show details about a container: type, live version, status, port, timestamps, TTL, and volume.

**Flags:**
- `--name` — container name (skips interactive selection)
- `--ping` — also run a connectivity check using the stored credentials

```bash
mkdb info app
mkdb info app --ping
```

### `mkdb stop [name]`

Stop a running container. The container and its data are preserved; use `mkdb start` to run it again.

### `mkdb start [name]`

Start a stopped container. If the underlying Docker container no longer exists, it is
recreated from the stored settings and credentials (its data volume is reattached).

### `mkdb restart [name]`

Restart a container, recreating it from stored settings if needed.

### `mkdb rm [name]` (alias `remove`)

Delete a container and its volume. This cannot be undone.

**Flags:**
- `--name` — container name (skips interactive selection)
- `--yes`, `-y` — skip the confirmation prompt (required in non-interactive use)

```bash
mkdb rm app
mkdb rm app --yes
```

### `mkdb creds show|copy|rotate [name]`

Manage credentials for the default user.

- `show` — print the `DB_URL=...` connection string to stdout
- `copy` — copy the connection string to the clipboard
- `rotate` — generate a new password, apply it in the container, and re-store it

```bash
# Print / pipe / capture
mkdb creds show app
mkdb creds show app >> .env
DB_URL=$(mkdb creds show app)

mkdb creds copy app
mkdb creds rotate app
```

Rotation is only available for authenticated databases.

### `mkdb user add|rm [name]`

Add or remove additional database users. Not supported for Redis.

**Flags:**
- `--name` — container name (skips interactive selection)
- `--username` — the user to add or remove (prompted/selected if omitted)
- `--yes`, `-y` — (`rm` only) skip the confirmation prompt

```bash
mkdb user add app --username appuser
mkdb user rm app --username appuser --yes
```

### `mkdb extend [name]`

Extend a container's TTL. If the container has already expired, the new expiry is measured
from now; otherwise the hours are added to the existing expiry.

**Flags:**
- `--name` — container name (skips interactive selection)
- `--hours` — hours to extend by (default: 24)

```bash
mkdb extend app
mkdb extend app --hours 48
```

### `mkdb config [name]`

Open a container's config file in `$EDITOR` (default `vi`). Config files live in
`~/.local/share/mkdb/configs/<name>/` and are mounted into the container; restart the
container to apply changes.

- **PostgreSQL**: `postgresql.conf`
- **MySQL**: `my.cnf`
- **Redis**: `redis.conf`

### `mkdb cleanup`

Review expired containers and interactively choose which to extend or remove. Removing a
container deletes its volume and record.

**Flags:**
- `--dry-run` — list expired containers without removing anything
- `--yes`, `-y` — remove all expired containers without prompting (for scripts/agents)

mkdb does not clean up automatically. When expired containers exist, other commands print a
one-line notice (`⚠ N database(s) expired — run 'mkdb cleanup' to review`) so nothing is
deleted without your involvement.

```bash
mkdb cleanup                 # interactive review
mkdb cleanup --dry-run       # show what would be removed
mkdb cleanup --yes           # remove all expired, no prompt
```

### `mkdb doctor`

Diagnose the local environment: Docker connectivity, data-directory access, encryption key,
the state database, and drift between Docker and mkdb's own records. Supports `--json`.

```bash
mkdb doctor
mkdb doctor --json
```

Exit codes: `0` all checks pass, `3` Docker is unreachable, `1` another check failed.

### `mkdb version`

Print the mkdb version. Also available as the `mkdb --version` / `mkdb -v` flags.

## Container lifecycle

1. **Create** — `mkdb create` builds and starts a container.
2. **Stop** — `mkdb stop` stops it; the container and data are preserved.
3. **Start** — `mkdb start` runs a stopped container again (recreating it if it was pruned).
4. **Restart** — `mkdb restart` bounces a container.
5. **Remove** — `mkdb rm` deletes the container and its volume.
6. **Cleanup** — `mkdb cleanup` removes expired containers you select.

## Configuration

### Data storage

All state lives under `XDG_DATA_HOME` (defaults to `~/.local/share/mkdb`):

```
~/.local/share/mkdb/
├── mkdb.db              # SQLite database tracking containers and users
├── mkdb.log             # Application logs
├── last_settings.json   # Last used settings for --repeat
├── .encryption.key      # Encryption key for stored passwords
├── configs/             # Per-database configuration files
│   └── app/
│       └── postgresql.conf
└── volumes/             # mkdb-managed named volumes
    └── app/
```

### Database type aliases

- **PostgreSQL**: `postgres`, `pg`, `postgresql`
- **MySQL**: `mysql`, `mariadb`
- **Redis**: `redis`

Aliases work anywhere a type is accepted (the `create` argument, `ls --type`, etc.).

### Default ports

- PostgreSQL: 5432
- MySQL: 3306
- Redis: 6379

If no `--port` is given and the default is in use, mkdb picks the next free port. If Docker
reports a conflict after the initial check, mkdb cleans up the partial container and retries
with the next available port. If a specific `--port` is given and it's taken, mkdb reports an
error.

### TTL (time to live)

Containers expire after their TTL (default: 2 hours). Expiry is surfaced as a notice on
other commands; run `mkdb cleanup` to act on it. Both running and stopped containers are
subject to expiry so stopped databases don't leak their volumes.

```bash
mkdb create postgres --name test --ttl 1     # short-lived
mkdb create postgres --name dev --ttl 168     # a week
mkdb extend dev --hours 48                     # keep it longer
```

## Volume options

1. **none** — no persistent storage (data is lost on removal)
2. **named** — stored in `~/.local/share/mkdb/volumes/<name>` and deleted on `rm`/cleanup
3. **custom path** — a bind mount to a host path you provide (left in place on removal)

## Connection strings

Printed as `DB_URL=<url>` on stdout:

**PostgreSQL**
```
DB_URL=postgresql://dbuser:<password>@localhost:5432/app   # authenticated
DB_URL=postgresql://postgres@localhost:5432/app            # --no-auth
```

**MySQL**
```
DB_URL=mysql://dbuser:<password>@tcp(localhost:3306)/app   # authenticated
DB_URL=mysql://root@tcp(localhost:3306)/app                # --no-auth
```

**Redis**
```
DB_URL=redis://:<password>@localhost:6379/0                # authenticated
DB_URL=redis://localhost:6379/0                            # --no-auth
```

## Scripting

Connection strings are written to stdout while all status messages, prompts, and logs go to
stderr, so piping and capturing are safe:

```bash
DB_URL=$(mkdb creds show app)          # captures only the connection string
mkdb create redis --name cache --no-auth --volume none | grep '^DB_URL='
```

Commands never prompt without a terminal. If required input is missing in a non-interactive
context, the command exits with an error naming the flag to pass.

## Automation & AI agents

mkdb is designed to be driven by scripts and AI agents, not just humans.

### JSON output

Pass `--json` to `create`, `ls`, `info`, `creds show`, `cleanup`, and `doctor` for a stable,
machine-readable document on stdout (human status still goes to stderr):

```bash
mkdb create postgres --name app --json
mkdb ls --json
mkdb info app --ping --json
```

A container object looks like:

```json
{
  "name": "app",
  "type": "postgres",
  "version": "18",
  "status": "running",
  "host": "localhost",
  "port": "5432",
  "url": "postgresql://dbuser:...@localhost:5432/app",
  "ready": true,
  "created_at": "2026-07-10T23:31:35-04:00",
  "expires_at": "2026-07-11T00:31:35-04:00",
  "expires_in_seconds": 3598,
  "volume_type": "none"
}
```

`ls` and `cleanup --yes`/`--dry-run` return an array of these objects. `ready` is present on
`create` and on `info --ping`. Fields are additive-only.

### Exit codes

| Code | Meaning |
| ---- | ------- |
| `0`  | success |
| `1`  | general error |
| `2`  | container/resource not found |
| `3`  | Docker daemon unreachable |
| `4`  | operation timed out (e.g. readiness wait) |

### Readiness

`create` blocks until the database accepts connections, so the returned `url` is ready to use
with no arbitrary `sleep`. Use `--wait-timeout` to bound the wait and `--no-wait` to skip it.

### Skill for AI agents

A ready-to-use agent skill lives in [`skills/mkdb/SKILL.md`](skills/mkdb/SKILL.md). It teaches
an agent to spin databases up and down on demand, seed them, and clean up afterward. Install it
by copying the folder into your agent's skills directory, for example:

```bash
# Claude Code
cp -r skills/mkdb ~/.claude/skills/mkdb

# Generic (~/.agents) convention
cp -r skills/mkdb ~/.agents/skills/mkdb
```

## Troubleshooting

### Docker daemon not running

Commands that only read local state (`version`, `ls`, `info`) work while Docker is down.
Commands that touch containers report a clear connection error — start Docker and retry.

### Port already in use

```
Error: port 5433 is already in use
```

Use a different `--port`, or omit `--port` to let mkdb find a free one automatically.

### Container name already exists

```
Error: a database named 'app' already exists
```

Choose another name or remove the existing container with `mkdb rm app`.

## Development

### Using mise (recommended)

This project uses [mise](https://mise.jdx.dev/) for task automation and Go version management.

```bash
mise run test      # run tests
mise run coverage  # tests with an HTML coverage report
mise run build     # build (runs tests first)
mise run quick     # build without tests
mise run dev       # install to $GOPATH/bin
mise run install   # install to ~/.local/bin
mise run fmt       # gofmt -w .
mise run lint      # go vet + gofmt check
mise run check     # test + lint + build
mise run run -- create postgres --name app
mise run clean     # remove build artifacts
```

### Manual development

```bash
go build -ldflags="-s -w" -o mkdb .
go test ./...
go test ./... -cover
```

### Project structure

```
mkdb/
├── cmd/                 # Cobra commands (create, start, stop, ...)
│   ├── root.go
│   ├── common.go        # shared container resolution & helpers
│   └── ...
├── internal/
│   ├── config/          # config, XDG paths, logging, encryption
│   ├── database/        # SQLite operations
│   ├── docker/          # Docker client wrapper
│   ├── credentials/     # password + connection-string helpers
│   ├── cleanup/         # expiry notice + interactive cleanup
│   ├── adapters/        # per-engine behavior (postgres, mysql, redis)
│   ├── types/           # shared constants and normalization
│   └── ui/              # prompts (huh) and output styling
├── skills/
│   └── mkdb/            # agent skill (SKILL.md)
├── mise.toml
└── main.go
```

## License

MIT

## Contributing

Contributions are welcome — please open an issue or pull request.
