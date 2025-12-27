package volumes

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/database"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"bytes", 500, "500 B"},
		{"kilobytes", 1024, "1.0 KB"},
		{"megabytes", 1024 * 1024, "1.0 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.0 GB"},
		{"mixed", 1536, "1.5 KB"},
		{"large", 5 * 1024 * 1024 * 1024, "5.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatSize(%d) = %v, want %v", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestScanOrphaned(t *testing.T) {
	// Initialize config and database for testing
	if err := config.Initialize(); err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}
	if err := database.Initialize(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create a test volume directory
	testVolumeName := "test-orphaned-volume"
	testVolumePath := filepath.Join(config.VolumesDir, testVolumeName)

	// Clean up any existing test volume
	os.RemoveAll(testVolumePath)

	// Create test volume
	if err := os.MkdirAll(testVolumePath, 0755); err != nil {
		t.Fatalf("Failed to create test volume: %v", err)
	}
	defer os.RemoveAll(testVolumePath)

	// Create a test file in the volume
	testFile := filepath.Join(testVolumePath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Scan for orphaned volumes
	orphaned, err := ScanOrphaned()
	if err != nil {
		t.Fatalf("ScanOrphaned() error: %v", err)
	}

	// Check if our test volume was found
	found := false
	for _, vol := range orphaned {
		if vol.Name == testVolumeName {
			found = true

			// Verify volume properties
			if vol.Path != testVolumePath {
				t.Errorf("Volume path = %v, want %v", vol.Path, testVolumePath)
			}
			if vol.Size == 0 {
				t.Error("Volume size should not be 0")
			}
			if vol.ModTime.IsZero() {
				t.Error("Volume ModTime should not be zero")
			}

			break
		}
	}

	if !found {
		t.Errorf("Test volume %s not found in orphaned volumes", testVolumeName)
	}
}

func TestScanOrphanedWithActiveContainer(t *testing.T) {
	// Initialize config and database
	if err := config.Initialize(); err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}
	if err := database.Initialize(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create a test volume
	testVolumeName := "test-active-volume"
	testVolumePath := filepath.Join(config.VolumesDir, testVolumeName)
	os.RemoveAll(testVolumePath)

	if err := os.MkdirAll(testVolumePath, 0755); err != nil {
		t.Fatalf("Failed to create test volume: %v", err)
	}
	defer os.RemoveAll(testVolumePath)

	// Create a container record that uses this volume
	container := &database.Container{
		Name:        "mkdb-" + testVolumeName,
		DisplayName: testVolumeName,
		Type:        "postgres",
		Version:     "15",
		Status:      "running",
		Port:        "5432",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		VolumeType:  "named",
		VolumePath:  testVolumeName,
	}

	if err := database.CreateContainer(container); err != nil {
		t.Fatalf("Failed to create test container: %v", err)
	}
	defer database.DeleteContainer(container.ID)

	// Scan for orphaned volumes
	orphaned, err := ScanOrphaned()
	if err != nil {
		t.Fatalf("ScanOrphaned() error: %v", err)
	}

	// Verify our active volume is NOT in the orphaned list
	for _, vol := range orphaned {
		if vol.Name == testVolumeName {
			t.Errorf("Active volume %s should not be in orphaned list", testVolumeName)
		}
	}
}

func TestGetDirSize(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create some test files
	testFiles := map[string]int{
		"file1.txt": 100,
		"file2.txt": 200,
		"file3.txt": 300,
	}

	totalSize := int64(0)
	for name, size := range testFiles {
		data := make([]byte, size)
		if err := os.WriteFile(filepath.Join(tmpDir, name), data, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		totalSize += int64(size)
	}

	// Calculate directory size
	calculatedSize, err := getDirSize(tmpDir)
	if err != nil {
		t.Fatalf("getDirSize() error: %v", err)
	}

	if calculatedSize != totalSize {
		t.Errorf("getDirSize() = %d, want %d", calculatedSize, totalSize)
	}
}

func TestOrphanedVolumeWithOriginalContainer(t *testing.T) {
	// Initialize config and database
	if err := config.Initialize(); err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}
	if err := database.Initialize(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create a test volume
	testVolumeName := "test-orphaned-with-metadata"
	testVolumePath := filepath.Join(config.VolumesDir, testVolumeName)
	os.RemoveAll(testVolumePath)

	if err := os.MkdirAll(testVolumePath, 0755); err != nil {
		t.Fatalf("Failed to create test volume: %v", err)
	}
	defer os.RemoveAll(testVolumePath)

	// Create an expired container record (so it won't show as active)
	container := &database.Container{
		Name:        "mkdb-" + testVolumeName,
		DisplayName: testVolumeName,
		Type:        "redis",
		Version:     "7.0",
		Status:      "expired",
		Port:        "6379",
		CreatedAt:   time.Now().Add(-48 * time.Hour),
		ExpiresAt:   time.Now().Add(-24 * time.Hour), // Expired
		VolumeType:  "named",
		VolumePath:  testVolumeName,
	}

	if err := database.CreateContainer(container); err != nil {
		t.Fatalf("Failed to create test container: %v", err)
	}
	defer database.DeleteContainer(container.ID)

	// Scan for orphaned volumes
	orphaned, err := ScanOrphaned()
	if err != nil {
		t.Fatalf("ScanOrphaned() error: %v", err)
	}

	// Find our test volume
	var testVol *OrphanedVolume
	for _, vol := range orphaned {
		if vol.Name == testVolumeName {
			testVol = vol
			break
		}
	}

	if testVol == nil {
		t.Fatalf("Test volume %s not found", testVolumeName)
	}

	// Verify it has the original container metadata
	if testVol.Container == nil {
		t.Error("Expected Container metadata to be present")
	} else {
		if testVol.Container.Type != "redis" {
			t.Errorf("Container.Type = %v, want redis", testVol.Container.Type)
		}
		if testVol.Container.Version != "7.0" {
			t.Errorf("Container.Version = %v, want 7.0", testVol.Container.Version)
		}
	}
}
