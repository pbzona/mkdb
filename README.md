# mkdb

A CLI tool to easily create and manage local Docker database containers for development environments.

## Features

- **Multiple Database Types**: Support for PostgreSQL, MySQL, and Redis
- **Simple Interface**: Interactive menus with vim keybindings or non-interactive with `--name` flag
- **Automatic Cleanup**: Containers expire after 2 hours by default (configurable)
- **Secure Credentials**: Encrypted password storage using AES-256-GCM
- **Automatic Volume Management**: Volumes created automatically, preserved on stop, removed on cleanup
- **User Management**: Create and manage additional database users
- **Connection Strings**: Auto-generated connection strings in environment variable format
- **TTL Management**: Extend container lifetime to prevent automatic cleanup

## Prerequisites

- Go 1.21 or later
- Docker installed and running
- Docker daemon accessible (Docker Desktop or Docker Engine)

## Installation

### Pre-built Binaries (Recommended)

Download the latest release for your platform from the [Releases page](https://github.com/pbzona/mkdb/releases):

- **Linux (amd64)**: `mkdb-linux-amd64`
- **Linux (arm64)**: `mkdb-linux-arm64`
- **macOS (Intel)**: `mkdb-darwin-amd64`
- **macOS (Apple Silicon)**: `mkdb-darwin-arm64`

```bash
# Example: Install on macOS (Apple Silicon)
curl -L https://github.com/pbzona/mkdb/releases/latest/download/mkdb-darwin-arm64 -o mkdb
chmod +x mkdb
sudo mv mkdb /usr/local/bin/
```

### From Source

```bash
git clone https://github.com/pbzona/mkdb.git
cd mkdb
go build -ldflags="-s -w" -o mkdb .
sudo mv mkdb /usr/local/bin/
```

The `-ldflags="-s -w"` flags strip debug symbols to reduce binary size (~30% smaller).

### Using Go Install

```bash
go install github.com/pbzona/mkdb@latest
```

## Quick Start

1. **Create a new database**:
```bash
mkdb start
```
Follow the interactive prompts to select database type, name, and volume options.

2. **List your databases**:
```bash
mkdb list
# or use the alias
mkdb ls
```

3. **View database details**:
```bash
mkdb info
```

4. **Get connection credentials**:
```bash
mkdb creds get
# or copy to clipboard
mkdb creds copy
```

5. **Stop a database** (preserves data):
```bash
mkdb stop
```

6. **Remove a database** (deletes data):
```bash
mkdb rm
# or
mkdb remove
```

## Commands

### `mkdb start`

Create a new database container.

**Flags:**
- `--db` - Database type (postgres/pg, mysql, redis)
- `--name` - Database name
- `--version` - Database version (default: postgres=18, mysql=latest, redis=latest)
- `--port` - Host port to bind to (default: database default port)
- `--volume` - Volume configuration: "none", "named", or a custom path (optional)
- `--ttl` - Time to live in hours (default: 2)
- `--repeat` - Use settings from last database created
- `--no-auth` - Create database without authentication (no username/password)

**Smart Prompting:**
- Only prompts for values not provided via flags
- Prompts for authentication preference if `--no-auth` flag not specified
- Remembers last used settings
- Use `--repeat` flag to quickly create another database with same settings

**Port Handling:**
- If no `--port` is specified and the default port is in use, mkdb will automatically find the next available port
- If `--port` is specified and that port is in use, an error will be returned
- Automatic port selection checks up to 100 ports from the default

**Examples:**
```bash
# Interactive mode (prompts for all options)
mkdb start

# Fully automated with flags (no prompts)
mkdb start --db postgres --name mydb --version 15 --port 5433

# Partially automated (prompts only for missing values)
mkdb start --db mysql --name testdb

# With custom TTL (expires in 48 hours instead of 24)
mkdb start --db postgres --name longdb --ttl 48

# Quick repeat of last database
mkdb start --repeat

# No persistent storage (data lost when container removed)
mkdb start --db redis --name cache --volume none

# Named volume (stored in ~/.local/share/mkdb/volumes/<name>)
mkdb start --db postgres --name mydb --volume named

# Custom volume path (bind mount)
mkdb start --db redis --name cache --volume /data/redis

# Short-lived test database (expires in 2 hours)
mkdb start --db postgres --name testdb --ttl 2

# Create database without authentication
mkdb start --db postgres --name publicdb --no-auth
```

**Default Credentials:**
- Username: `dbuser`
- Password: Randomly generated 12-character alphanumeric string (displayed after creation)

**Unauthenticated Access:**

You can create databases without authentication in two ways:

1. **Using the `--no-auth` flag:**
```bash
# PostgreSQL without authentication (uses trust mode)
mkdb start --db postgres --name devdb --no-auth

# MySQL without authentication (allows root login without password)
mkdb start --db mysql --name devdb --no-auth

# Redis without authentication (no requirepass)
mkdb start --db redis --name cache --no-auth
```

2. **Interactive prompt:** When you don't specify the `--no-auth` flag, mkdb will ask "Enable authentication? (recommended)" - answer `n` or `N` to create without authentication.

**Note:** Unauthenticated databases cannot use password rotation (`mkdb creds rotate`). Connection strings for unauthenticated databases will not include credentials. This is useful for local development or testing scenarios where security is not a concern.

### `mkdb list` / `mkdb ls`

List all database containers with optional filtering.

**Flags:**
- `--type` - Filter by database type (postgres, mysql, redis)
- `--status` - Filter by status (running, stopped, expired)

**Examples:**
```bash
# List all containers
mkdb list

# Use the shorter alias
mkdb ls

# Filter by database type (accepts: postgres, pg, mysql, redis)
mkdb ls --type postgres

# Filter by status (accepts: running/up, stopped/down, expired)
mkdb ls --status running

# Combine filters
mkdb ls --type redis --status running
```

**Output Format:**

The list command displays containers in a formatted table with:
- Name
- Type (postgres, mysql, redis)
- Status (running, stopped, expired)
- Port
- TTL remaining

### `mkdb stop`

Stop a running container while preserving its data.

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb stop

# Non-interactive mode
mkdb stop --name mydb
```

### `mkdb restart`

Restart a stopped container with its existing data.

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb restart

# Non-interactive mode
mkdb restart --name mydb
```

### `mkdb config`

Edit the database configuration file in your default editor (`$EDITOR`).

Configuration files are automatically created and stored in `~/.local/share/mkdb/configs/<dbname>/` when you create a database. Each database type has its own config file:
- **PostgreSQL**: `postgresql.conf`
- **MySQL**: `my.cnf`
- **Redis**: `redis.conf`

```bash
# Edit config (uses $EDITOR, defaults to vi)
mkdb config

# Then restart to apply changes
mkdb restart
```

**Example workflow:**
```bash
# Edit PostgreSQL config
export EDITOR=nano
mkdb config
# Make your changes, save and exit
# Restart to apply
mkdb restart
```

### `mkdb remove` / `mkdb rm`

Delete a container and its volume permanently.

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb remove

# Non-interactive mode
mkdb remove --name mydb

# or use the shorter alias
mkdb rm --name mydb
```

### `mkdb info`

Display detailed information about a container including:
- Database type and version
- Status (running/stopped)
- Port mappings
- Created and expiration dates
- Time remaining before auto-cleanup
- Volume information

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb info

# Non-interactive mode
mkdb info --name mydb
```

### `mkdb creds get`

Display the connection string for the default user.

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb creds get

# Non-interactive mode
mkdb creds get --name mydb

# Pipe to .env file
mkdb creds get --name mydb >> .env

# Use in a script
DB_URL=$(mkdb creds get --name mydb)
```

**Output format:**
```
DB_URL=postgresql://dbuser:Xy9k2mN8pL4v@localhost:5432/mydb
```

### `mkdb creds copy`

Copy the connection string to clipboard.

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb creds copy

# Non-interactive mode
mkdb creds copy --name mydb
```

### `mkdb creds rotate`

Generate a new password for the default user and update it in the database.

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb creds rotate

# Non-interactive mode
mkdb creds rotate --name mydb
```

### `mkdb user create`

Create a new database user with a generated password.

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb user create

# Non-interactive mode
mkdb user create --name mydb
```

### `mkdb user delete`

Delete a non-default database user.

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb user delete

# Non-interactive mode
mkdb user delete --name mydb
```

### `mkdb extend`

Extend the TTL of a container to prevent automatic cleanup.

**Flags:**
- `--name` - Container name (skips interactive selection)
- `--hours` - Number of hours to extend (default: 1)

```bash
# Interactive mode, extend by 1 hour
mkdb extend

# Non-interactive mode, extend by 1 hour
mkdb extend --name mydb

# Extend by custom hours
mkdb extend --name mydb --hours 24
```

### `mkdb test` / `mkdb ping`

Test database connectivity by running a simple query.

**Flags:**
- `--name` - Container name (skips interactive selection)

```bash
# Interactive mode
mkdb test

# Non-interactive mode
mkdb test --name mydb

# or use the alias
mkdb ping --name mydb
```

This command will:
- Execute a test query specific to the database type
- Display the connection status and query results

**What it tests:**
- **PostgreSQL**: Runs `SELECT 1 as status, current_user, current_database();`
- **MySQL**: Runs `SELECT 1 as status, USER() as user, DATABASE() as db;`
- **Redis**: Runs `PING`

### `mkdb cleanup`

Remove expired database containers and their volumes.

```bash
mkdb cleanup
```

This command will:
- Find all expired containers
- Interactively prompt you to select which ones to remove
- Delete both the container and its volume
- Remove the container record from the database

The cleanup check also runs automatically every time you execute any mkdb command.

### `mkdb version`

Display the current version of mkdb.

```bash
mkdb version
# or use the flag
mkdb --version
```

## Container Lifecycle

mkdb follows a simple container lifecycle model:

1. **Create**: `mkdb start` - Creates a new container with persistent volume
2. **Stop**: `mkdb stop` - Stops the container, preserves data
3. **Restart**: `mkdb restart` - Restarts a stopped container with existing data
4. **Remove**: `mkdb remove` - Permanently deletes container and volume
5. **Cleanup**: `mkdb cleanup` - Automatically removes expired containers and volumes

**Volume Management:**
- Volumes are automatically created when you start a container
- Volumes are preserved when you stop a container (use `restart` to start again)
- Volumes are deleted when you explicitly remove a container
- Volumes are deleted when a container expires and is cleaned up

## Configuration

### Data Storage

All data is stored in your XDG_DATA_HOME directory (defaults to `~/.local/share/mkdb`):

```
~/.local/share/mkdb/
├── mkdb.db              # SQLite database tracking containers
├── mkdb.log             # Application logs
├── last_settings.json   # Last used settings for --repeat
├── .encryption.key      # Encryption key for passwords
├── configs/             # Database configuration files
│   ├── mydb/
│   │   └── postgresql.conf
│   └── cache/
│       └── redis.conf
└── volumes/             # Named volumes storage
    └── mydb/            # Example named volume
```

**Configuration Files:**

Each database container gets its own configuration directory with a default config file that you can edit using `mkdb config`. The config files are automatically mounted into the containers and changes take effect after restarting the container.

### Database Type Aliases

For convenience, mkdb accepts multiple aliases for database types:

- **PostgreSQL**: `postgres`, `pg`, `postgresql`
- **MySQL**: `mysql`, `mariadb`
- **Redis**: `redis`

These work in all commands (`--db`, `--type` filters, etc.)

### Default Ports

- PostgreSQL: 5432
- MySQL: 3306
- Redis: 6379

### TTL (Time to Live)

Containers are automatically cleaned up after their TTL expires (default: 2 hours). The cleanup check runs every time you execute a mkdb command.

**Configuring TTL:**
- Use `--ttl` flag when creating: `mkdb start --db postgres --name mydb --ttl 48`
- Default TTL: 2 hours
- Extend TTL of existing container: `mkdb extend --name mydb --hours 1`

When a container expires:
- The container is stopped and removed
- The volume is **removed** (deleted permanently)
- The container record is deleted from the database

**Examples:**
```bash
# Short-lived test database (1 hour)
mkdb start --db postgres --name test --ttl 1

# Long-lived development database (7 days)
mkdb start --db postgres --name dev --ttl 168

# Extend expiration of existing database by 1 hour
mkdb extend --name mydb

# Extend by more hours
mkdb extend --name mydb --hours 24
```

## Volume Options

When creating a database, you have three volume options:

1. **None** - No persistent storage (data lost when container is removed)
2. **Named** - Volume stored in `~/.local/share/mkdb/volumes/<name>`
3. **Custom Path** - Volume at a specific filesystem path (bind mount)

## Connection Strings

Connection strings are provided in the format:

**PostgreSQL:**
```
# With authentication (random password generated)
DB_URL=postgresql://dbuser:<random-password>@localhost:5432/mydb

# Without authentication (--no-auth)
DB_URL=postgresql://postgres@localhost:5432/mydb
```

**MySQL:**
```
# With authentication (random password generated)
DB_URL=mysql://dbuser:<random-password>@tcp(localhost:3306)/mydb

# Without authentication (--no-auth)
DB_URL=mysql://root@tcp(localhost:3306)/mydb
```

**Redis:**
```
# With authentication (random password generated)
DB_URL=redis://:<random-password>@localhost:6379/0

# Without authentication (--no-auth)
DB_URL=redis://localhost:6379/0
```

## Interactive Navigation

All menus support both arrow keys and vim keybindings:

- `j` or `↓` - Move down
- `k` or `↑` - Move up
- `Enter` - Select
- `Ctrl+C` - Cancel

## Troubleshooting

### Docker daemon not running

```
Error: failed to connect to Docker daemon
```

**Solution:** Start Docker Desktop or ensure Docker daemon is running.

### Port already in use

**When using default port:**
If the default port is in use, mkdb will automatically find and use the next available port:
```
⚠ Default port 5432 is in use, finding next available port...
ℹ Using port 5433
```

**When specifying a custom port:**
```
Error: port 5433 is already in use (use default port for automatic selection)
```

**Solution:** Either use a different port or omit the `--port` flag for automatic selection:
```bash
# Let mkdb find an available port automatically
mkdb start

# Or specify a different port
mkdb start --port 5434
```

### Container name already exists

```
Error: container with name 'mydb' already exists
```

**Solution:** Choose a different name or remove the existing container with `mkdb rm`.

### Permission denied on volume path

```
Error: failed to create volume directory
```

**Solution:** Ensure you have write permissions to the specified volume path.

## Examples

### Quick start with flags (no prompts)

```bash
$ mkdb start --db postgres --name devdb
ℹ Creating postgres database 'devdb'...
✓ Database 'devdb' created successfully!

╭──────────────────────────────────────────────────────────╮
│ DB_URL=postgresql://dbuser:Xy9k2mN8pL4v@localhost:5432/devdb │
╰──────────────────────────────────────────────────────────╯

ℹ Database will expire in 2 hours (at 2025-12-23 17:00:00)
ℹ Use 'mkdb start --repeat' to quickly create another database with the same settings
```

### Custom TTL for longer-lived database

```bash
$ mkdb start --db postgres --name devdb --ttl 48
ℹ Creating postgres database 'devdb'...
✓ Database 'devdb' created successfully!

╭──────────────────────────────────────────────────────────╮
│ DB_URL=postgresql://dbuser:Bw7n5pK2mL9t@localhost:5432/devdb │
╰──────────────────────────────────────────────────────────╯

ℹ Database will expire in 48 hours (at 2025-12-25 17:00:00)
```

### Repeat last settings

```bash
$ mkdb start --repeat
ℹ Using previous settings: postgres database 'devdb'
? Continue with these settings? Yes

ℹ Creating postgres database 'devdb'...
✓ Database 'devdb' created successfully!
```

### Interactive mode (original behavior)

```bash
$ mkdb start
? Select database type: postgres
? Enter database name: devdb
? Enable authentication? (recommended) Yes
? Do you want to create a volume for this database? named

✓ Database 'devdb' created successfully!

╭──────────────────────────────────────────────────────────╮
│ DB_URL=postgresql://dbuser:M3kP9xL2vN7w@localhost:5432/devdb │
╰──────────────────────────────────────────────────────────╯

ℹ Database will expire in 2 hours (at 2025-12-23 17:00:00)
```

### Extend TTL before expiration

```bash
$ mkdb extend --name devdb --hours 24
✓ Container 'devdb' TTL extended by 24 hours!
ℹ New expiration: 2025-12-26 15:00:00
```

### Create additional user

```bash
$ mkdb user create
? Select container: devdb (postgres)
? Enter username: appuser
ℹ Generating password...

✓ User 'appuser' created successfully!

╭────────────────────────────────────────────────────────────╮
│ DB_URL=postgresql://appuser:Xy9k2...@localhost:5432/devdb │
╰────────────────────────────────────────────────────────────╯
```

### Rotate credentials

```bash
$ mkdb creds rotate
? Select container: devdb (postgres)
ℹ Generating new password...

✓ Password rotated successfully!

╭──────────────────────────────────────────────────────────╮
│ DB_URL=postgresql://dbuser:Bw8m5...@localhost:5432/devdb │
╰──────────────────────────────────────────────────────────╯
```

## Development

### Using mise (Recommended)

This project uses [mise](https://mise.jdx.dev/) for task automation and Go version management.

**Install mise:**
```bash
curl https://mise.run | sh
```

**Available tasks:**

```bash
# Run tests
mise run test

# Run tests with coverage report
mise run coverage

# Build the binary (runs tests first)
mise run build

# Quick build without tests (for rapid iteration)
mise run quick

# Install to $GOPATH/bin for local testing
mise run dev

# Install to /usr/local/bin (requires sudo)
mise run install

# Format Go files
mise run fmt

# Run linting checks (go vet, gofmt)
mise run lint

# Run all checks (test, lint, build)
mise run check

# Build and run with arguments
mise run run -- start --db postgres

# Clean build artifacts
mise run clean
```

**Example workflow:**
```bash
# Make changes to code
mise run fmt              # Format code
mise run test             # Run tests
mise run dev              # Install locally
mkdb start --db postgres  # Test the changes

# Before committing
mise run check            # Run all checks
```

### Manual Development (without mise)

**Building from source:**

```bash
git clone https://github.com/pbzona/mkdb.git
cd mkdb
go build -ldflags="-s -w" -o mkdb .
```

**Running tests:**

```bash
go test ./...

# With coverage
go test ./... -cover
```

**Installing locally:**

```bash
# Install to $GOPATH/bin
go install .

# Or install to /usr/local/bin
go build -ldflags="-s -w" -o mkdb .
sudo mv mkdb /usr/local/bin/
```

### Project structure

```
mkdb/
├── cmd/                 # Cobra commands
│   ├── root.go
│   ├── start.go
│   ├── stop.go
│   ├── restart.go
│   ├── rm.go
│   ├── info.go
│   ├── creds.go
│   ├── user.go
│   ├── extend.go
│   ├── test.go
│   ├── cleanup.go
│   └── ...
├── internal/
│   ├── config/          # Configuration and encryption
│   ├── database/        # SQLite operations
│   ├── docker/          # Docker client wrapper
│   ├── credentials/     # Password generation
│   ├── cleanup/         # TTL enforcement
│   ├── adapters/        # Database adapters (postgres, mysql, redis)
│   └── ui/              # Terminal UI components
├── mise.toml            # mise task configuration
└── main.go
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

If you encounter any issues or have questions, please file an issue on GitHub.
