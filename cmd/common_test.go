package cmd

import (
	"testing"

	"github.com/pbzona/mkdb/internal/database"
)

func TestConnectionString(t *testing.T) {
	tests := []struct {
		name      string
		container *database.Container
		username  string
		password  string
		want      string
	}{
		{
			name:      "postgres authenticated uses display name",
			container: &database.Container{Type: "postgres", Port: "5432", DisplayName: "app"},
			username:  "dbuser",
			password:  "secret",
			want:      "DB_URL=postgresql://dbuser:secret@localhost:5432/app",
		},
		{
			name:      "postgres unauthenticated",
			container: &database.Container{Type: "postgres", Port: "5432", DisplayName: "app"},
			want:      "DB_URL=postgresql://postgres@localhost:5432/app",
		},
		{
			name:      "redis always uses database index 0",
			container: &database.Container{Type: "redis", Port: "6379", DisplayName: "cache"},
			password:  "secret",
			want:      "DB_URL=redis://:secret@localhost:6379/0",
		},
		{
			name:      "mysql authenticated",
			container: &database.Container{Type: "mysql", Port: "3306", DisplayName: "app"},
			username:  "dbuser",
			password:  "secret",
			want:      "DB_URL=mysql://dbuser:secret@tcp(localhost:3306)/app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := connectionString(tt.container, tt.username, tt.password)
			if got != tt.want {
				t.Errorf("connectionString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHumanizeHours(t *testing.T) {
	tests := []struct {
		hours int
		want  string
	}{
		{1, "1 hour"},
		{2, "2 hours"},
		{24, "24 hours"},
	}

	for _, tt := range tests {
		if got := humanizeHours(tt.hours); got != tt.want {
			t.Errorf("humanizeHours(%d) = %q, want %q", tt.hours, got, tt.want)
		}
	}
}
