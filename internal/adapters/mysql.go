package adapters

import (
	"fmt"
	"strings"
)

// MySQLAdapter implements the DatabaseAdapter interface for MySQL
type MySQLAdapter struct{}

func NewMySQLAdapter() *MySQLAdapter {
	return &MySQLAdapter{}
}

func (m *MySQLAdapter) GetName() string {
	return "mysql"
}

func (m *MySQLAdapter) GetAliases() []string {
	return []string{"mysql", "mariadb"}
}

func (m *MySQLAdapter) GetImage(version string) string {
	if version == "" {
		version = "latest"
	}
	return fmt.Sprintf("mysql:%s", version)
}

func (m *MySQLAdapter) GetDefaultPort() string {
	return "3306"
}

func (m *MySQLAdapter) GetEnvVars(dbName, username, password string) []string {
	envVars := []string{
		fmt.Sprintf("MYSQL_DATABASE=%s", dbName),
	}

	// If username and password are empty, allow unauthenticated root login
	if username != "" && password != "" {
		envVars = append(envVars,
			fmt.Sprintf("MYSQL_USER=%s", username),
			fmt.Sprintf("MYSQL_PASSWORD=%s", password),
			"MYSQL_ROOT_PASSWORD=rootpassword",
		)
	} else {
		// Allow empty root password for unauthenticated access
		envVars = append(envVars, "MYSQL_ALLOW_EMPTY_PASSWORD=yes")
	}

	return envVars
}

func (m *MySQLAdapter) GetDataPath() string {
	return "/var/lib/mysql"
}

func (m *MySQLAdapter) GetConfigPath() string {
	return "/etc/mysql/conf.d"
}

func (m *MySQLAdapter) GetConfigFileName() string {
	return "my.cnf"
}

func (m *MySQLAdapter) GetDefaultConfig() string {
	return `# MySQL configuration file
# Managed by mkdb
# Edit with: mkdb config

[mysqld]
# Connection Settings
max_connections = 100

# Logging
general_log = 1
general_log_file = /var/log/mysql/general.log
`
}

func (m *MySQLAdapter) CreateUserCommand(username, password, dbName string) []string {
	return []string{
		"mysql", "-u", "root", "-prootpassword", "-e",
		fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%%'; FLUSH PRIVILEGES;",
			username, password, dbName, username),
	}
}

func (m *MySQLAdapter) DeleteUserCommand(username, dbName string) []string {
	return []string{
		"mysql", "-u", "root", "-prootpassword", "-e",
		fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'; FLUSH PRIVILEGES;", username),
	}
}

func (m *MySQLAdapter) RotatePasswordCommand(username, newPassword, dbName string) []string {
	return []string{
		"mysql", "-u", "root", "-prootpassword", "-e",
		fmt.Sprintf("ALTER USER '%s'@'%%' IDENTIFIED BY '%s'; FLUSH PRIVILEGES;", username, newPassword),
	}
}

func (m *MySQLAdapter) FormatConnectionString(username, password, host, port, dbName string) string {
	// If no username/password, connect as root without authentication
	if username == "" && password == "" {
		return fmt.Sprintf("mysql://root@tcp(%s:%s)/%s", host, port, dbName)
	}
	return fmt.Sprintf("mysql://%s:%s@tcp(%s:%s)/%s", username, password, host, port, dbName)
}

func (m *MySQLAdapter) SupportsUsername() bool {
	return true
}

func (m *MySQLAdapter) SupportsUnauthenticated() bool {
	return true
}

func (m *MySQLAdapter) GetCommandArgs(password string) []string {
	// MySQL uses environment variables, no custom command needed
	return []string{}
}

func (m *MySQLAdapter) GetVersionCommand() []string {
	return []string{"mysqld", "--version"}
}

func (m *MySQLAdapter) ParseVersion(output string) string {
	// Input: "mysqld  Ver 8.0.35 for Linux on x86_64 (MySQL Community Server - GPL)"
	// Output: "8.0.35"

	// Look for "Ver X.Y.Z"
	parts := strings.Fields(output)
	for i, part := range parts {
		if part == "Ver" && i+1 < len(parts) {
			version := parts[i+1]
			// Remove any trailing characters
			if idx := strings.Index(version, "-"); idx != -1 {
				version = version[:idx]
			}
			return version
		}
	}

	return strings.TrimSpace(output)
}
