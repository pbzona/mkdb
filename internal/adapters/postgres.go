package adapters

import (
	"fmt"
	"strings"
)

// PostgresAdapter implements the DatabaseAdapter interface for PostgreSQL
type PostgresAdapter struct{}

func NewPostgresAdapter() *PostgresAdapter {
	return &PostgresAdapter{}
}

func (p *PostgresAdapter) GetName() string {
	return "postgres"
}

func (p *PostgresAdapter) GetAliases() []string {
	return []string{"pg", "postgres", "postgresql"}
}

func (p *PostgresAdapter) GetImage(version string) string {
	if version == "" {
		version = "18"
	}
	return fmt.Sprintf("postgres:%s", version)
}

func (p *PostgresAdapter) GetDefaultPort() string {
	return "5432"
}

func (p *PostgresAdapter) GetEnvVars(dbName, username, password string) []string {
	return []string{
		fmt.Sprintf("POSTGRES_DB=%s", dbName),
		fmt.Sprintf("POSTGRES_USER=%s", username),
		fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
		"PGDATA=/var/lib/postgresql/data",
	}
}

func (p *PostgresAdapter) GetDataPath() string {
	return "/var/lib/postgresql"
}

func (p *PostgresAdapter) GetConfigPath() string {
	return "/etc/postgresql"
}

func (p *PostgresAdapter) GetConfigFileName() string {
	return "postgresql.conf"
}

func (p *PostgresAdapter) GetDefaultConfig() string {
	return `# PostgreSQL configuration file
# Managed by mkdb
# Edit with: mkdb config

# Connection Settings
max_connections = 100
shared_buffers = 128MB

# Logging
logging_collector = on
log_directory = 'log'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
log_statement = 'all'
`
}

func (p *PostgresAdapter) CreateUserCommand(username, password, dbName string) []string {
	return []string{
		"psql", "-U", "dbuser", "-d", dbName, "-c",
		fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'; GRANT ALL PRIVILEGES ON DATABASE %s TO %s;",
			username, password, dbName, username),
	}
}

func (p *PostgresAdapter) DeleteUserCommand(username, dbName string) []string {
	return []string{
		"psql", "-U", "dbuser", "-d", dbName, "-c",
		fmt.Sprintf("DROP USER IF EXISTS %s;", username),
	}
}

func (p *PostgresAdapter) RotatePasswordCommand(username, newPassword, dbName string) []string {
	return []string{
		"psql", "-U", "dbuser", "-d", dbName, "-c",
		fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s';", username, newPassword),
	}
}

func (p *PostgresAdapter) FormatConnectionString(username, password, host, port, dbName string) string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", username, password, host, port, dbName)
}

func (p *PostgresAdapter) SupportsUsername() bool {
	return true
}

func (p *PostgresAdapter) GetCommandArgs(password string) []string {
	// PostgreSQL uses environment variables, no custom command needed
	return []string{}
}

func (p *PostgresAdapter) GetVersionCommand() []string {
	return []string{"postgres", "--version"}
}

func (p *PostgresAdapter) ParseVersion(output string) string {
	// Input: "postgres (PostgreSQL) 16.1 (Debian 16.1-1.pgdg120+1)"
	// Output: "16.1"
	// Simple parsing: look for version pattern
	// Format is typically: "postgres (PostgreSQL) X.Y ..."
	// We'll use a simpler approach: split and find the version number

	// Split by spaces and find the version number after "PostgreSQL"
	parts := strings.Fields(output)
	for i, part := range parts {
		if part == "(PostgreSQL)" && i+1 < len(parts) {
			// Next part is the version
			version := parts[i+1]
			// Remove any trailing characters that aren't part of the version
			if idx := strings.Index(version, "-"); idx != -1 {
				version = version[:idx]
			}
			return version
		}
	}

	// Fallback: return the output as-is
	return strings.TrimSpace(output)
}
