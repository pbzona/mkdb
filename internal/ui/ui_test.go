package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/pbzona/mkdb/internal/database"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "Negative duration",
			duration: -1 * time.Hour,
			want:     "expired",
		},
		{
			name:     "Less than 1 hour",
			duration: 30 * time.Minute,
			want:     "0h 30m",
		},
		{
			name:     "Exactly 1 hour",
			duration: 1 * time.Hour,
			want:     "1h 0m",
		},
		{
			name:     "Multiple hours",
			duration: 5*time.Hour + 45*time.Minute,
			want:     "5h 45m",
		},
		{
			name:     "Exactly 24 hours",
			duration: 24 * time.Hour,
			want:     "24h 0m",
		},
		{
			name:     "More than 24 hours",
			duration: 25*time.Hour + 30*time.Minute,
			want:     "1d 1h 30m",
		},
		{
			name:     "Multiple days",
			duration: 72*time.Hour + 15*time.Minute,
			want:     "3d 0h 15m",
		},
		{
			name:     "Large duration",
			duration: 100*time.Hour + 45*time.Minute,
			want:     "4d 4h 45m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintContainerInfo(t *testing.T) {
	// This test just verifies that PrintContainerInfo doesn't panic
	// We can't easily test the output without mocking fmt.Println

	now := time.Now()
	container := &database.Container{
		ID:          1,
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "postgres",
		Version:     "15",
		ContainerID: "abc123",
		Port:        "5432",
		Status:      "running",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
		VolumeType:  "named",
		VolumePath:  "testdb",
	}

	// Should not panic
	PrintContainerInfo(container)
}

func TestPrintContainerInfoNoVolume(t *testing.T) {
	now := time.Now()
	container := &database.Container{
		ID:          1,
		Name:        "mkdb-testdb",
		DisplayName: "testdb",
		Type:        "redis",
		Version:     "7",
		ContainerID: "xyz789",
		Port:        "6379",
		Status:      "running",
		CreatedAt:   now,
		ExpiresAt:   now.Add(12 * time.Hour),
		VolumeType:  "",
		VolumePath:  "",
	}

	// Should not panic
	PrintContainerInfo(container)
}

func TestFormatVolumeInfo(t *testing.T) {
	tests := []struct {
		name       string
		container  *database.Container
		wantPrefix string
	}{
		{
			name: "No volume",
			container: &database.Container{
				VolumeType: "",
				VolumePath: "",
			},
			wantPrefix: "none",
		},
		{
			name: "Named volume",
			container: &database.Container{
				VolumeType: "named",
				VolumePath: "testdb",
			},
			wantPrefix: "testdb (named)",
		},
		{
			name: "Bind mount",
			container: &database.Container{
				VolumeType: "bind",
				VolumePath: "/path/to/data",
			},
			wantPrefix: "/path/to/data (bind)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatVolumeInfo(tt.container)
			if got != tt.wantPrefix {
				t.Errorf("formatVolumeInfo() = %v, want %v", got, tt.wantPrefix)
			}
		})
	}
}

func TestStyleFunctions(t *testing.T) {
	// These tests verify that the style functions don't panic
	// and return non-empty strings

	testMessage := "test message"

	tests := []struct {
		name string
		fn   func(string)
	}{
		{"Success", Success},
		{"Error", Error},
		{"Warning", Warning},
		{"Info", Info},
		{"Header", Header},
		{"Box", Box},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			tt.fn(testMessage)
		})
	}
}

func TestLipglossStyles(t *testing.T) {
	// Verify that styles can render text without panicking
	testText := "test"

	// These should not panic and should return non-empty strings
	if successStyle.Render(testText) == "" {
		t.Error("successStyle.Render() returned empty string")
	}

	if errorStyle.Render(testText) == "" {
		t.Error("errorStyle.Render() returned empty string")
	}

	if warningStyle.Render(testText) == "" {
		t.Error("warningStyle.Render() returned empty string")
	}

	if infoStyle.Render(testText) == "" {
		t.Error("infoStyle.Render() returned empty string")
	}

	if headerStyle.Render(testText) == "" {
		t.Error("headerStyle.Render() returned empty string")
	}

	if boxStyle.Render(testText) == "" {
		t.Error("boxStyle.Render() returned empty string")
	}
}

func TestSelectContainerError(t *testing.T) {
	// Test with empty container list
	_, err := SelectContainer([]*database.Container{}, "Select container")
	if err == nil {
		t.Error("SelectContainer() with empty list should return error")
	}

	expectedMsg := "no containers found"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("SelectContainer() error = %v, want error containing %q", err, expectedMsg)
	}
}

func TestSelectUserError(t *testing.T) {
	// Test with empty user list
	_, err := SelectUser([]*database.User{}, "Select user")
	if err == nil {
		t.Error("SelectUser() with empty list should return error")
	}

	expectedMsg := "no users found"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("SelectUser() error = %v, want error containing %q", err, expectedMsg)
	}
}
