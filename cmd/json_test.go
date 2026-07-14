package cmd

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/pbzona/mkdb/internal/database"
)

func TestContainerToJSON(t *testing.T) {
	now := time.Now()
	c := &database.Container{
		DisplayName: "app",
		Type:        "postgres",
		Version:     "18",
		Status:      "running",
		Port:        "5432",
		CreatedAt:   now,
		ExpiresAt:   now.Add(time.Hour),
	}

	ready := true
	out := containerToJSON(c, "postgresql://dbuser:pw@localhost:5432/app", &ready)

	if out.Name != "app" || out.Type != "postgres" || out.Port != "5432" {
		t.Errorf("unexpected identity fields: %+v", out)
	}
	if out.Host != "localhost" {
		t.Errorf("Host = %q, want localhost", out.Host)
	}
	if out.URL != "postgresql://dbuser:pw@localhost:5432/app" {
		t.Errorf("URL = %q", out.URL)
	}
	if out.Ready == nil || !*out.Ready {
		t.Errorf("Ready = %v, want true", out.Ready)
	}
	// Empty volume type normalizes to "none".
	if out.VolumeType != "none" {
		t.Errorf("VolumeType = %q, want none", out.VolumeType)
	}
	if out.ExpiresInSeconds <= 0 {
		t.Errorf("ExpiresInSeconds = %d, want > 0", out.ExpiresInSeconds)
	}
}

func TestContainerToJSON_ReadyOmittedWhenNil(t *testing.T) {
	c := &database.Container{DisplayName: "app", Type: "redis", Port: "6379", ExpiresAt: time.Now().Add(time.Hour)}
	b, err := json.Marshal(containerToJSON(c, "", nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["ready"]; ok {
		t.Errorf("ready should be omitted when nil, got %s", b)
	}
	if _, ok := m["url"]; ok {
		t.Errorf("url should be omitted when empty, got %s", b)
	}
	// expires_in_seconds is always present even when zero-ish.
	if _, ok := m["expires_in_seconds"]; !ok {
		t.Errorf("expires_in_seconds should always be present, got %s", b)
	}
}

func TestConnectivityProbe(t *testing.T) {
	tests := []struct {
		name          string
		dbType        string
		dbName        string
		adminUser     string
		adminPassword string
		want          []string
		wantErr       bool
	}{
		{
			name:      "postgres with admin user",
			dbType:    "postgres",
			dbName:    "app",
			adminUser: "dbuser",
			want:      []string{"psql", "-U", "dbuser", "-d", "app", "-c", "SELECT 1;"},
		},
		{
			name:   "postgres unauthenticated falls back to postgres role",
			dbType: "postgres",
			dbName: "app",
			want:   []string{"psql", "-U", "postgres", "-d", "app", "-c", "SELECT 1;"},
		},
		{
			name:          "mysql with password",
			dbType:        "mysql",
			dbName:        "app",
			adminPassword: "secret",
			want:          []string{"mysql", "-u", "root", "-psecret", "-e", "SELECT 1;"},
		},
		{
			name:   "mysql without password",
			dbType: "mysql",
			dbName: "app",
			want:   []string{"mysql", "-u", "root", "-e", "SELECT 1;"},
		},
		{
			name:          "redis with password",
			dbType:        "redis",
			adminPassword: "secret",
			want:          []string{"redis-cli", "-a", "secret", "PING"},
		},
		{
			name:   "redis without password",
			dbType: "redis",
			want:   []string{"redis-cli", "PING"},
		},
		{
			name:    "unknown type errors",
			dbType:  "mongodb",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := connectivityProbe(tt.dbType, tt.dbName, tt.adminUser, tt.adminPassword)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("connectivityProbe() = %v, want %v", got, tt.want)
			}
		})
	}
}
