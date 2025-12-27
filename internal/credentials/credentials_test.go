package credentials

import (
	"strings"
	"testing"
)

func TestGeneratePassword(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"32 characters", 32},
		{"16 characters", 16},
		{"64 characters", 64},
		{"1 character", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password, err := GeneratePassword(tt.length)
			if err != nil {
				t.Fatalf("GeneratePassword() error = %v", err)
			}

			if len(password) != tt.length {
				t.Errorf("GeneratePassword() length = %d, want %d", len(password), tt.length)
			}

			// Check that password only contains alphanumeric characters
			for _, char := range password {
				if !strings.ContainsRune(charset, char) {
					t.Errorf("GeneratePassword() contains invalid character: %c", char)
				}
			}
		})
	}
}

func TestGeneratePasswordRandomness(t *testing.T) {
	// Generate multiple passwords and ensure they're different
	passwords := make(map[string]bool)
	iterations := 10

	for i := 0; i < iterations; i++ {
		password, err := GeneratePassword(32)
		if err != nil {
			t.Fatalf("GeneratePassword() error = %v", err)
		}

		if passwords[password] {
			t.Errorf("GeneratePassword() generated duplicate password")
		}
		passwords[password] = true
	}
}

func TestFormatConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		dbType   string
		username string
		password string
		host     string
		port     string
		dbName   string
		want     string
	}{
		{
			name:     "PostgreSQL",
			dbType:   "postgres",
			username: "testuser",
			password: "testpass",
			host:     "localhost",
			port:     "5432",
			dbName:   "testdb",
			want:     "postgresql://testuser:testpass@localhost:5432/testdb",
		},
		{
			name:     "MySQL",
			dbType:   "mysql",
			username: "testuser",
			password: "testpass",
			host:     "localhost",
			port:     "3306",
			dbName:   "testdb",
			want:     "mysql://testuser:testpass@tcp(localhost:3306)/testdb",
		},
		{
			name:     "Redis with username and password",
			dbType:   "redis",
			username: "testuser",
			password: "testpass",
			host:     "localhost",
			port:     "6379",
			dbName:   "",
			want:     "redis://:testpass@localhost:6379/0",
		},
		{
			name:     "Redis with password only",
			dbType:   "redis",
			username: "",
			password: "testpass",
			host:     "localhost",
			port:     "6379",
			dbName:   "",
			want:     "redis://:testpass@localhost:6379/0",
		},
		{
			name:     "Redis without auth",
			dbType:   "redis",
			username: "",
			password: "",
			host:     "localhost",
			port:     "6379",
			dbName:   "",
			want:     "redis://localhost:6379/0",
		},
		{
			name:     "Redis with specific database number",
			dbType:   "redis",
			username: "",
			password: "testpass",
			host:     "localhost",
			port:     "6379",
			dbName:   "5",
			want:     "redis://:testpass@localhost:6379/5",
		},
		{
			name:     "Unknown database type",
			dbType:   "unknown",
			username: "testuser",
			password: "testpass",
			host:     "localhost",
			port:     "1234",
			dbName:   "testdb",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatConnectionString(tt.dbType, tt.username, tt.password, tt.host, tt.port, tt.dbName)
			if got != tt.want {
				t.Errorf("FormatConnectionString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatEnvVar(t *testing.T) {
	tests := []struct {
		name             string
		connectionString string
		want             string
	}{
		{
			name:             "PostgreSQL connection string",
			connectionString: "postgresql://user:pass@localhost:5432/db",
			want:             "DB_URL=postgresql://user:pass@localhost:5432/db",
		},
		{
			name:             "Empty connection string",
			connectionString: "",
			want:             "DB_URL=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatEnvVar(tt.connectionString)
			if got != tt.want {
				t.Errorf("FormatEnvVar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultUsername != "dbuser" {
		t.Errorf("DefaultUsername = %v, want dbuser", DefaultUsername)
	}

	if DefaultPassword != "$uper$ecret" {
		t.Errorf("DefaultPassword = %v, want $uper$ecret", DefaultPassword)
	}

	// Verify charset contains expected characters
	expectedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if charset != expectedChars {
		t.Errorf("charset = %v, want %v", charset, expectedChars)
	}
}
