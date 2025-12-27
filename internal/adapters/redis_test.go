package adapters

import (
	"testing"
)

func TestRedisAdapter_GetCommandArgs(t *testing.T) {
	adapter := NewRedisAdapter()

	tests := []struct {
		name     string
		password string
		want     []string
	}{
		{
			name:     "with password",
			password: "secret123",
			want:     []string{"redis-server", "--requirepass", "secret123"},
		},
		{
			name:     "without password",
			password: "",
			want:     []string{},
		},
		{
			name:     "with special characters in password",
			password: "$uper$ecret",
			want:     []string{"redis-server", "--requirepass", "$uper$ecret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.GetCommandArgs(tt.password)
			if len(got) != len(tt.want) {
				t.Errorf("GetCommandArgs() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GetCommandArgs()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRedisAdapter_FormatConnectionString(t *testing.T) {
	adapter := NewRedisAdapter()

	tests := []struct {
		name     string
		username string
		password string
		host     string
		port     string
		dbName   string
		want     string
	}{
		{
			name:     "with password default db",
			username: "",
			password: "secret",
			host:     "localhost",
			port:     "6379",
			dbName:   "",
			want:     "redis://:secret@localhost:6379/0",
		},
		{
			name:     "with password and db number",
			username: "",
			password: "secret",
			host:     "localhost",
			port:     "6379",
			dbName:   "5",
			want:     "redis://:secret@localhost:6379/5",
		},
		{
			name:     "without password",
			username: "",
			password: "",
			host:     "localhost",
			port:     "6379",
			dbName:   "",
			want:     "redis://localhost:6379/0",
		},
		{
			name:     "username is ignored",
			username: "ignored",
			password: "secret",
			host:     "localhost",
			port:     "6379",
			dbName:   "",
			want:     "redis://:secret@localhost:6379/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.FormatConnectionString(tt.username, tt.password, tt.host, tt.port, tt.dbName)
			if got != tt.want {
				t.Errorf("FormatConnectionString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRedisAdapter_SupportsUsername(t *testing.T) {
	adapter := NewRedisAdapter()
	if adapter.SupportsUsername() {
		t.Error("Redis should not support username authentication")
	}
}

func TestRedisAdapter_GetEnvVars(t *testing.T) {
	adapter := NewRedisAdapter()

	// Redis should not use environment variables for authentication
	envVars := adapter.GetEnvVars("testdb", "user", "pass")
	if len(envVars) != 0 {
		t.Errorf("GetEnvVars() should return empty slice, got %v", envVars)
	}
}
