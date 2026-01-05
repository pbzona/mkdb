package adapters

// DatabaseAdapter defines the interface that all database providers must implement
type DatabaseAdapter interface {
	// GetName returns the canonical name of the database (e.g., "postgres", "mysql", "redis")
	GetName() string

	// GetAliases returns alternative names that map to this database type
	GetAliases() []string

	// GetImage returns the Docker image for the specified version
	GetImage(version string) string

	// GetDefaultPort returns the default port for this database
	GetDefaultPort() string

	// GetEnvVars returns the environment variables needed to configure the container
	// Pass empty strings for username and password to run in unauthenticated mode
	GetEnvVars(dbName, username, password string) []string

	// SupportsUnauthenticated returns whether this database can run without authentication
	SupportsUnauthenticated() bool

	// GetDataPath returns the path inside the container where data is stored
	GetDataPath() string

	// GetConfigPath returns the path inside the container where config files are stored
	GetConfigPath() string

	// GetConfigFileName returns the name of the main configuration file
	GetConfigFileName() string

	// GetDefaultConfig returns the default configuration file content
	GetDefaultConfig() string

	// CreateUserCommand returns the command to create a new user in the database
	// Returns nil if user creation is not supported
	CreateUserCommand(username, password, dbName string) []string

	// DeleteUserCommand returns the command to delete a user from the database
	// Returns nil if user deletion is not supported
	DeleteUserCommand(username, dbName string) []string

	// RotatePasswordCommand returns the command to rotate a user's password
	// Returns nil if password rotation is not supported
	RotatePasswordCommand(username, newPassword, dbName string) []string

	// FormatConnectionString returns the connection string for this database
	FormatConnectionString(username, password, host, port, dbName string) string

	// SupportsUsername returns whether this database supports username authentication
	SupportsUsername() bool

	// GetCommandArgs returns custom command line arguments for starting the container
	// Returns empty slice if no custom command is needed
	// Pass empty string for password to run in unauthenticated mode
	GetCommandArgs(password string) []string

	// GetVersionCommand returns the command to get the database version
	// Returns nil if version detection is not supported
	GetVersionCommand() []string

	// ParseVersion parses the version output from GetVersionCommand
	// Returns a clean version string (e.g., "16.1" instead of full output)
	ParseVersion(output string) string
}
