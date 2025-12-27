# Database Adapters

This package implements an adapter pattern for supporting multiple database providers in mkdb.

## Overview

The adapter system allows for easy addition of new database providers without modifying core application logic. Each database type is implemented as an adapter that conforms to the `DatabaseAdapter` interface.

## Architecture

### Components

1. **DatabaseAdapter Interface** (`adapter.go`)
   - Defines the contract that all database providers must implement
   - Includes methods for configuration, Docker setup, and database operations

2. **Concrete Adapters**
   - `PostgresAdapter` (`postgres.go`) - PostgreSQL support
   - `MySQLAdapter` (`mysql.go`) - MySQL/MariaDB support
   - `RedisAdapter` (`redis.go`) - Redis support

3. **Registry** (`registry.go`)
   - Manages all registered adapters
   - Provides lookup by name or alias
   - Thread-safe singleton pattern

## Adding a New Database Provider

To add support for a new database (e.g., MongoDB), follow these steps:

### 1. Create the Adapter Implementation

Create a new file `mongodb.go`:

```go
package adapters

import "fmt"

type MongoDBAdapter struct{}

func NewMongoDBAdapter() *MongoDBAdapter {
    return &MongoDBAdapter{}
}

func (m *MongoDBAdapter) GetName() string {
    return "mongodb"
}

func (m *MongoDBAdapter) GetAliases() []string {
    return []string{"mongodb", "mongo"}
}

func (m *MongoDBAdapter) GetImage(version string) string {
    if version == "" {
        version = "latest"
    }
    return fmt.Sprintf("mongo:%s", version)
}

func (m *MongoDBAdapter) GetDefaultPort() string {
    return "27017"
}

func (m *MongoDBAdapter) GetEnvVars(dbName, username, password string) []string {
    return []string{
        fmt.Sprintf("MONGO_INITDB_DATABASE=%s", dbName),
        fmt.Sprintf("MONGO_INITDB_ROOT_USERNAME=%s", username),
        fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", password),
    }
}

func (m *MongoDBAdapter) GetDataPath() string {
    return "/data/db"
}

func (m *MongoDBAdapter) GetConfigPath() string {
    return "/etc/mongo"
}

func (m *MongoDBAdapter) GetConfigFileName() string {
    return "mongod.conf"
}

func (m *MongoDBAdapter) GetDefaultConfig() string {
    return `# MongoDB configuration file
# Managed by mkdb

storage:
  dbPath: /data/db

net:
  bindIp: 0.0.0.0
  port: 27017
`
}

func (m *MongoDBAdapter) CreateUserCommand(username, password, dbName string) []string {
    return []string{
        "mongo", dbName, "--eval",
        fmt.Sprintf("db.createUser({user: '%s', pwd: '%s', roles: [{role: 'readWrite', db: '%s'}]})",
            username, password, dbName),
    }
}

func (m *MongoDBAdapter) DeleteUserCommand(username, dbName string) []string {
    return []string{
        "mongo", dbName, "--eval",
        fmt.Sprintf("db.dropUser('%s')", username),
    }
}

func (m *MongoDBAdapter) RotatePasswordCommand(username, newPassword, dbName string) []string {
    return []string{
        "mongo", dbName, "--eval",
        fmt.Sprintf("db.changeUserPassword('%s', '%s')", username, newPassword),
    }
}
```

### 2. Register the Adapter

Update the `GetRegistry()` function in `registry.go` to register your new adapter:

```go
func GetRegistry() *Registry {
    once.Do(func() {
        defaultRegistry = &Registry{
            adapters:    make(map[string]DatabaseAdapter),
            aliasToName: make(map[string]string),
        }
        // Register default adapters
        defaultRegistry.Register(NewPostgresAdapter())
        defaultRegistry.Register(NewMySQLAdapter())
        defaultRegistry.Register(NewRedisAdapter())
        defaultRegistry.Register(NewMongoDBAdapter()) // Add this line
    })
    return defaultRegistry
}
```

### 3. That's It!

The adapter will automatically be:
- Available in all database type selections
- Handled by all Docker operations
- Included in connection string generation
- Managed by configuration systems

## Interface Methods

### Required Methods

| Method | Purpose | Returns |
|--------|---------|---------|
| `GetName()` | Canonical database name | string |
| `GetAliases()` | Alternative names/aliases | []string |
| `GetImage(version)` | Docker image with version | string |
| `GetDefaultPort()` | Default connection port | string |
| `GetEnvVars(db, user, pass)` | Environment variables for container | []string |
| `GetDataPath()` | Data directory in container | string |
| `GetConfigPath()` | Config directory in container | string |
| `GetConfigFileName()` | Main config file name | string |
| `GetDefaultConfig()` | Default config file content | string |

### Optional Methods (can return nil)

| Method | Purpose | Returns |
|--------|---------|---------|
| `CreateUserCommand(user, pass, db)` | Command to create database user | []string or nil |
| `DeleteUserCommand(user, db)` | Command to delete database user | []string or nil |
| `RotatePasswordCommand(user, pass, db)` | Command to change user password | []string or nil |

If these methods return `nil`, the operation will return an error indicating it's not supported for this database type.

## Usage Examples

### Getting an Adapter

```go
import "github.com/pbzona/mkdb/internal/adapters"

// Get the global registry
registry := adapters.GetRegistry()

// Get adapter by canonical name
adapter, err := registry.Get("postgres")

// Get adapter by alias
adapter, err := registry.Get("pg")
adapter, err := registry.Get("postgresql")
```

### Using an Adapter

```go
// Get configuration
image := adapter.GetImage("15")
port := adapter.GetDefaultPort()
envVars := adapter.GetEnvVars("mydb", "user", "pass")

// Get paths
dataPath := adapter.GetDataPath()
configPath := adapter.GetConfigPath()

// Get user management commands
createCmd := adapter.CreateUserCommand("newuser", "password", "mydb")
if createCmd != nil {
    // Execute the command
}
```

### Listing Available Databases

```go
registry := adapters.GetRegistry()
dbTypes := registry.List() // ["postgres", "mysql", "redis"]
```

### Normalizing Database Types

```go
registry := adapters.GetRegistry()
canonical, err := registry.NormalizeType("pg") // Returns "postgres"
```

## Testing

When adding a new adapter, add test cases to `registry_test.go`:

```go
{
    name:      "mongodb by name",
    dbType:    "mongodb",
    wantName:  "mongodb",
    wantError: false,
},
{
    name:      "mongodb by alias",
    dbType:    "mongo",
    wantName:  "mongodb",
    wantError: false,
},
```

Run tests:

```bash
go test ./internal/adapters/...
```

## Benefits of the Adapter Pattern

1. **Extensibility**: Add new database types without modifying existing code
2. **Maintainability**: Database-specific logic is isolated in adapters
3. **Testability**: Each adapter can be tested independently
4. **Consistency**: All databases follow the same interface contract
5. **Type Safety**: Compile-time checking ensures all methods are implemented
6. **Discovery**: Registry provides runtime discovery of available databases

## Database-Specific Implementations

### Redis

Redis has some unique characteristics that are handled differently:

1. **Authentication**: Redis doesn't use traditional username/password authentication. It only uses a password (requirepass). The adapter:
   - Returns `false` for `SupportsUsername()`
   - Uses command line args `--requirepass` to set the password
   - Formats connection strings as `redis://:<password>@host:port/db`

2. **Database Selection**: Redis uses numeric databases (0-15 by default). The `dbName` parameter is treated as the database number in the connection string.

3. **Connection String Format**: 
   - With password: `redis://:password@localhost:6379/0`
   - Without password: `redis://localhost:6379/0`
   - Note the `:` before the password (no username)

### PostgreSQL

- Uses environment variables for configuration
- Standard username/password authentication
- Connection string: `postgresql://user:pass@host:port/dbname`

### MySQL

- Uses environment variables for configuration
- Standard username/password authentication
- Connection string: `mysql://user:pass@tcp(host:port)/dbname`

## Design Decisions

### Why Not Use Type Switches?

The previous implementation used switch statements scattered throughout the codebase. The adapter pattern provides:
- Better separation of concerns
- Easier to add new providers
- Reduced coupling between components
- Single responsibility principle

### Why a Registry?

The registry pattern provides:
- Centralized management of adapters
- Runtime discovery of available databases
- Alias resolution
- Thread-safe access to adapters

### Why Return nil for Unsupported Operations?

Some databases don't support certain operations (e.g., Redis user management). Returning `nil` allows:
- Clear indication of unsupported features
- Graceful error handling
- Optional functionality without breaking the interface

### Why GetCommandArgs()?

Some databases (like Redis) need custom command line arguments to configure authentication, as they don't use environment variables. This method allows adapters to:
- Specify custom startup commands
- Configure authentication via command line
- Override default container behavior when needed
