# mkdb

A CLI tool to easily create and manage local Docker database containers for development environments.

## Features

- **Multiple Database Types**: Support for PostgreSQL, MySQL, and Redis
- **Simple Interface**: Interactive menus with vim keybindings
- **Automatic Cleanup**: Containers expire after 24 hours by default (configurable)
- **Secure Credentials**: Encrypted password storage using AES-256-GCM
- **Volume Management**: Named volumes or custom paths for data persistence
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
mkdb stat
```

4. **Get connection credentials**:
```bash
mkdb creds get
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
- `--version` - Database version (default: latest)
- `--port` - Host port to bind to (default: database default port)
- `--volume` - Volume configuration: "none", "named", or a custom path (optional)
- `--ttl` - Time to live in hours (default: 24)
- `--repeat` - Use settings from last database created

**Smart Prompting:**
- Only prompts for values not provided via flags
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
```

**Default Credentials:**
- Username: `dbuser`
- Password: `$uper$ecret`

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

Stop and remove a container (volume is preserved).

```bash
mkdb stop
```

### `mkdb restart`

Restart an existing container.

```bash
mkdb restart
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

```bash
mkdb remove
# or use the shorter alias
mkdb rm
```

### `mkdb stat`

Display detailed information about a container including:
- Database type and version
- Status (running/stopped)
- Port mappings
- Created and expiration dates
- Time remaining before auto-cleanup
- Volume information

```bash
mkdb stat
```

### `mkdb creds get`

Display the connection string for the default user.

**Flags:**
- `--copy` - Copy connection string to clipboard instead of displaying it

```bash
# Display connection string
mkdb creds get

# Copy to clipboard
mkdb creds get --copy

# Pipe to .env file
mkdb creds get >> .env

# Use in a script
DB_URL=$(mkdb creds get)
```

**Output format:**
```
DB_URL=postgresql://dbuser:$uper$ecret@localhost:5432/mydb
```

### `mkdb creds rotate`

Generate a new password for the default user and update it in the database.

**Flags:**
- `--copy` - Copy connection string to clipboard instead of displaying it

```bash
# Display new connection string
mkdb creds rotate

# Copy new credentials to clipboard
mkdb creds rotate --copy
```

### `mkdb user create`

Create a new database user with a generated password.

```bash
mkdb user create
```

### `mkdb user delete`

Delete a non-default database user.

```bash
mkdb user delete
```

### `mkdb extend`

Extend the TTL of a container to prevent automatic cleanup.

**Flags:**
- `--hours` - Number of hours to extend (default: 24)

```bash
# Extend by 24 hours
mkdb extend

# Extend by 48 hours
mkdb extend --hours 48
```

### `mkdb test` / `mkdb ping`

Test database connectivity by running a simple query.

```bash
mkdb test
# or use the alias
mkdb ping
```

This command will:
- Prompt you to select a container
- Execute a test query specific to the database type
- Display the connection status and query results

**What it tests:**
- **PostgreSQL**: Runs `SELECT 1 as status, current_user, current_database();`
- **MySQL**: Runs `SELECT 1 as status, USER() as user, DATABASE() as db;`
- **Redis**: Runs `PING`

### `mkdb version`

Display the current version of mkdb.

```bash
mkdb version
# or use the flag
mkdb --version
```

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

Containers are automatically cleaned up after their TTL expires (default: 24 hours). The cleanup check runs every time you execute a mkdb command.

**Configuring TTL:**
- Use `--ttl` flag when creating: `mkdb start --db postgres --name mydb --ttl 48`
- Default TTL: 24 hours
- Extend TTL of existing container: `mkdb extend --hours 24`

When a container expires:
- The container is stopped and removed
- The volume is **preserved** (not deleted)
- You can restart it later with `mkdb restart`

**Examples:**
```bash
# Short-lived test database (2 hours)
mkdb start --db postgres --name test --ttl 2

# Long-lived development database (7 days)
mkdb start --db postgres --name dev --ttl 168

# Extend expiration of existing database
mkdb extend --hours 48
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
DB_URL=postgresql://dbuser:$uper$ecret@localhost:5432/mydb
```

**MySQL:**
```
DB_URL=mysql://dbuser:$uper$ecret@tcp(localhost:3306)/mydb
```

**Redis:**
```
DB_URL=redis://:$uper$ecret@localhost:6379
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
│ DB_URL=postgresql://dbuser:$uper$ecret@localhost:5432/devdb │
╰──────────────────────────────────────────────────────────╯

ℹ Database will expire in 24 hours (at 2025-12-24 15:00:00)
ℹ Use 'mkdb start --repeat' to quickly create another database with the same settings
```

### Custom TTL for temporary database

```bash
$ mkdb start --db postgres --name tempdb --ttl 2
ℹ Creating postgres database 'tempdb'...
✓ Database 'tempdb' created successfully!

╭──────────────────────────────────────────────────────────╮
│ DB_URL=postgresql://dbuser:$uper$ecret@localhost:5432/tempdb │
╰──────────────────────────────────────────────────────────╯

ℹ Database will expire in 2 hours (at 2025-12-23 17:00:00)
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
? Do you want to create a volume for this database? named

✓ Database 'devdb' created successfully!

╭──────────────────────────────────────────────────────────╮
│ DB_URL=postgresql://dbuser:$uper$ecret@localhost:5432/devdb │
╰──────────────────────────────────────────────────────────╯

ℹ Database will expire in 24 hours (at 2025-12-24 15:00:00)
```

### Extend TTL before expiration

```bash
$ mkdb extend --hours 48
? Select container to extend TTL: devdb (postgres)

✓ Container 'devdb' TTL extended by 48 hours!
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
│   ├── stat.go
│   ├── creds.go
│   ├── user.go
│   └── extend.go
├── internal/
│   ├── config/          # Configuration and encryption
│   ├── database/        # SQLite operations
│   ├── docker/          # Docker client wrapper
│   ├── credentials/     # Password generation
│   ├── cleanup/         # TTL enforcement
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
