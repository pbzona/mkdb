package adapters

import (
	"fmt"
	"strings"
)

// RedisAdapter implements the DatabaseAdapter interface for Redis
type RedisAdapter struct{}

func NewRedisAdapter() *RedisAdapter {
	return &RedisAdapter{}
}

func (r *RedisAdapter) GetName() string {
	return "redis"
}

func (r *RedisAdapter) GetAliases() []string {
	return []string{"redis"}
}

func (r *RedisAdapter) GetImage(version string) string {
	if version == "" {
		version = "8"
	}
	return fmt.Sprintf("redis:%s", version)
}

func (r *RedisAdapter) GetDefaultPort() string {
	return "6379"
}

func (r *RedisAdapter) GetEnvVars(dbName, username, password string) []string {
	// Redis doesn't use environment variables for auth in the official image
	// Auth is configured via command line or redis.conf
	return []string{}
}

func (r *RedisAdapter) GetDataPath() string {
	return "/data"
}

func (r *RedisAdapter) GetConfigPath() string {
	return "/usr/local/etc/redis"
}

func (r *RedisAdapter) GetConfigFileName() string {
	return "redis.conf"
}

func (r *RedisAdapter) GetDefaultConfig() string {
	return `# Redis configuration file
# Managed by mkdb
# Edit with: mkdb config

# Network
bind 0.0.0.0
port 6379

# Logging
loglevel notice

# Authentication
# Password will be set dynamically via command line
`
}

func (r *RedisAdapter) CreateUserCommand(username, password, dbName string) []string {
	// Redis user management is more complex, not supported in basic adapter
	return nil
}

func (r *RedisAdapter) DeleteUserCommand(username, dbName string) []string {
	// Redis user management is more complex, not supported in basic adapter
	return nil
}

func (r *RedisAdapter) RotatePasswordCommand(username, newPassword, dbName string) []string {
	// Redis user management is more complex, not supported in basic adapter
	return nil
}

func (r *RedisAdapter) FormatConnectionString(username, password, host, port, dbName string) string {
	// Redis connection string format: redis://[user][:password]@host:port[/database]
	// Standard Redis doesn't use username (pre-Redis 6 ACLs)
	// Database number can be specified (0-15 by default)
	if password != "" {
		// Use default database 0 if no dbName specified
		db := "0"
		if dbName != "" {
			db = dbName
		}
		return fmt.Sprintf("redis://:%s@%s:%s/%s", password, host, port, db)
	}
	return fmt.Sprintf("redis://%s:%s/0", host, port)
}

func (r *RedisAdapter) SupportsUsername() bool {
	return false
}

func (r *RedisAdapter) SupportsUnauthenticated() bool {
	return true
}

// GetCommandArgs returns the command line arguments to start Redis with password
func (r *RedisAdapter) GetCommandArgs(password string) []string {
	// If password is empty, Redis will run without authentication
	if password != "" {
		return []string{"redis-server", "--requirepass", password}
	}
	return []string{}
}

func (r *RedisAdapter) GetVersionCommand() []string {
	return []string{"redis-server", "--version"}
}

func (r *RedisAdapter) ParseVersion(output string) string {
	// Input: "Redis server v=7.2.3 sha=00000000:0 malloc=jemalloc-5.3.0 bits=64 build=7504b1fedf883f2f"
	// Output: "7.2.3"

	// Look for "v=X.Y.Z"
	parts := strings.Fields(output)
	for _, part := range parts {
		if strings.HasPrefix(part, "v=") {
			version := strings.TrimPrefix(part, "v=")
			// Remove any trailing characters
			if idx := strings.Index(version, "-"); idx != -1 {
				version = version[:idx]
			}
			return version
		}
	}

	return strings.TrimSpace(output)
}
