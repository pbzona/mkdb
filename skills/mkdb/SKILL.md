---
name: mkdb
description: >-
  Spin up and tear down disposable local databases (PostgreSQL, MySQL, Redis) on
  demand using the mkdb CLI. Use whenever you need a real database for a task -
  e.g. "spin up a local postgres", "I need a test database", "give me a throwaway
  mysql", "create a dev redis", "load this schema into a database", or whenever
  you need a working connection string for local development, testing, or running
  migrations. Prefer this over hand-writing `docker run` for databases.
---

# mkdb: disposable local databases

`mkdb` creates and manages throwaway Docker database containers with one command.
It handles image pulls, port allocation, credential generation, readiness waiting,
schema seeding, TTL-based expiry, and cleanup. It supports **postgres**, **mysql**,
and **redis**.

Use it instead of raw `docker run` when the task needs a database: it returns a
ready-to-use connection string, waits until the DB actually accepts connections,
and tracks state so you can clean up reliably.

## Golden rules

1. **Always use `--json`** so you can parse output reliably. Data is on stdout;
   status text is on stderr.
2. **Always pass every flag** (`--name`, type, `--ttl`). mkdb never prompts when
   there's no TTY, but explicit flags avoid ambiguity.
3. **Always set a short `--ttl`** (1-2 hours) for task-scoped databases, and name
   them after the task (e.g. `--name pr-1234-test`).
4. **Always tear down** what you create when the task is done (`mkdb rm <name> --yes`).
5. **Check exit codes.** Non-zero means it failed - see the table below.

## Preflight

Confirm mkdb and Docker are working before relying on them:

```bash
mkdb version                 # not found? see "If mkdb isn't installed"
mkdb doctor --json           # checks Docker, data dir, state db; exit 3 = Docker down
```

Then discover what already exists so you reuse instead of duplicating:

```bash
mkdb ls --json
```

## Create a database

`create` blocks until the database accepts connections, so the returned `url` is
immediately usable - no `sleep` needed.

```bash
# Postgres, 1-hour TTL, machine-readable
mkdb create postgres --name myapp --ttl 1 --json
```

Example output (stdout):

```json
{
  "name": "myapp",
  "type": "postgres",
  "status": "running",
  "host": "localhost",
  "port": "5432",
  "url": "postgresql://dbuser:PASSWORD@localhost:5432/myapp",
  "ready": true,
  "expires_in_seconds": 3599,
  "volume_type": "none"
}
```

Parse the `url` field and use it directly. To capture just the URL in a shell:

```bash
URL=$(mkdb create postgres --name myapp --ttl 1 --json | jq -r .url)
```

Useful flags:

- `--version <v>` — image version (defaults: postgres=18, redis=8, mysql=latest)
- `--port <p>` — pin a host port (otherwise a free one is auto-selected)
- `--volume none|named|<path>` — persistence (default `none`; throwaway DBs want `none`)
- `--no-auth` — no password (only for quick throwaway cases)
- `--wait-timeout <secs>` — bound the readiness wait (default 30)
- `--no-wait` — return before the DB is ready (rarely what you want)

## Seed a schema at creation

Load a `.sql` file once the database is ready (postgres and mysql only):

```bash
mkdb create postgres --name myapp --ttl 1 --init ./schema.sql --json
```

If the script fails, `create` exits non-zero and reports the error. This is the
preferred way to get a schema in place - it's atomic with creation.

## Get a connection string later

```bash
mkdb creds show myapp --json | jq -r .url    # machine-readable
mkdb creds show myapp                          # prints DB_URL=... (stdout)
DB_URL=$(mkdb creds show myapp)                # capture the DB_URL= line
```

## Inspect and verify

```bash
mkdb info myapp --json            # details (status, port, TTL, url)
mkdb info myapp --ping --json     # also runs a real connectivity check -> "ready": true/false
```

## Keep a database alive longer

Task taking a while? Extend the TTL instead of letting it expire:

```bash
mkdb extend myapp --hours 4
```

## Tear down

```bash
mkdb rm myapp --yes               # remove one database + its volume
mkdb cleanup --dry-run --json     # list expired databases without touching them
mkdb cleanup --yes                # remove ALL expired databases
```

Always remove databases you created for a task once you're done.

## Exit codes

| Code | Meaning | What to do |
| ---- | ------- | ---------- |
| 0 | success | continue |
| 1 | general error | read stderr; fix and retry |
| 2 | not found | the named database doesn't exist; `mkdb ls --json` |
| 3 | Docker unreachable | tell the user to start Docker; `mkdb doctor` |
| 4 | readiness timeout | DB is slow; retry `mkdb info <name> --ping`, or raise `--wait-timeout` |

## Fallback: unsupported engines

mkdb only supports postgres, mysql, and redis. For anything else (mongodb,
clickhouse, etc.), fall back to `docker run` directly - mkdb won't help there.

## If mkdb isn't installed

Check for a Go toolchain and install, or point the user to prebuilt binaries:

```bash
go install github.com/pbzona/mkdb@latest      # if Go is available
```

Otherwise direct the user to https://github.com/pbzona/mkdb/releases for a
prebuilt binary for their platform. mkdb requires Docker to be installed and running.

## End-to-end example

```bash
# 1. Preflight
mkdb doctor --json || { echo "fix Docker first"; exit 1; }

# 2. Create + seed
URL=$(mkdb create postgres --name pr-1234 --ttl 1 --init ./schema.sql --json | jq -r .url)

# 3. Use $URL for the task (run migrations, tests, queries)...

# 4. Clean up
mkdb rm pr-1234 --yes
```
