package adapters

import (
	"testing"
)

func TestRegistry_Get(t *testing.T) {
	registry := GetRegistry()

	tests := []struct {
		name      string
		dbType    string
		wantName  string
		wantError bool
	}{
		{
			name:      "postgres by name",
			dbType:    "postgres",
			wantName:  "postgres",
			wantError: false,
		},
		{
			name:      "postgres by alias pg",
			dbType:    "pg",
			wantName:  "postgres",
			wantError: false,
		},
		{
			name:      "mysql by name",
			dbType:    "mysql",
			wantName:  "mysql",
			wantError: false,
		},
		{
			name:      "mysql by alias mariadb",
			dbType:    "mariadb",
			wantName:  "mysql",
			wantError: false,
		},
		{
			name:      "redis by name",
			dbType:    "redis",
			wantName:  "redis",
			wantError: false,
		},
		{
			name:      "invalid database type",
			dbType:    "mongodb",
			wantName:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := registry.Get(tt.dbType)
			if tt.wantError {
				if err == nil {
					t.Errorf("Get() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Get() unexpected error: %v", err)
				return
			}
			if adapter.GetName() != tt.wantName {
				t.Errorf("Get() got name = %v, want %v", adapter.GetName(), tt.wantName)
			}
		})
	}
}

func TestRegistry_NormalizeType(t *testing.T) {
	registry := GetRegistry()

	tests := []struct {
		name      string
		dbType    string
		want      string
		wantError bool
	}{
		{
			name:      "normalize pg to postgres",
			dbType:    "pg",
			want:      "postgres",
			wantError: false,
		},
		{
			name:      "normalize postgresql to postgres",
			dbType:    "postgresql",
			want:      "postgres",
			wantError: false,
		},
		{
			name:      "normalize mariadb to mysql",
			dbType:    "mariadb",
			want:      "mysql",
			wantError: false,
		},
		{
			name:      "invalid type",
			dbType:    "invalid",
			want:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.NormalizeType(tt.dbType)
			if tt.wantError {
				if err == nil {
					t.Errorf("NormalizeType() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NormalizeType() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistry_List(t *testing.T) {
	registry := GetRegistry()
	types := registry.List()

	if len(types) != 3 {
		t.Errorf("List() returned %d types, want 3", len(types))
	}

	expectedTypes := map[string]bool{
		"postgres": true,
		"mysql":    true,
		"redis":    true,
	}

	for _, dbType := range types {
		if !expectedTypes[dbType] {
			t.Errorf("List() returned unexpected type: %s", dbType)
		}
	}
}

func TestAdapters_Interface(t *testing.T) {
	registry := GetRegistry()

	tests := []struct {
		name   string
		dbType string
	}{
		{"postgres", "postgres"},
		{"mysql", "mysql"},
		{"redis", "redis"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := registry.Get(tt.dbType)
			if err != nil {
				t.Fatalf("Get() error: %v", err)
			}

			// Test all interface methods
			if adapter.GetName() == "" {
				t.Error("GetName() returned empty string")
			}
			if len(adapter.GetAliases()) == 0 {
				t.Error("GetAliases() returned empty slice")
			}
			if adapter.GetImage("latest") == "" {
				t.Error("GetImage() returned empty string")
			}
			if adapter.GetDefaultPort() == "" {
				t.Error("GetDefaultPort() returned empty string")
			}
			if adapter.GetDataPath() == "" {
				t.Error("GetDataPath() returned empty string")
			}
			if adapter.GetConfigPath() == "" {
				t.Error("GetConfigPath() returned empty string")
			}
			if adapter.GetConfigFileName() == "" {
				t.Error("GetConfigFileName() returned empty string")
			}
			if adapter.GetDefaultConfig() == "" {
				t.Error("GetDefaultConfig() returned empty string")
			}

			// Test env vars (some adapters may return empty slice)
			envVars := adapter.GetEnvVars("testdb", "testuser", "testpass")
			_ = envVars // Just ensure it doesn't panic
		})
	}
}
