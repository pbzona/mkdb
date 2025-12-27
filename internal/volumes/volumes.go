package volumes

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/database"
)

// OrphanedVolume represents a volume that exists on disk but has no active container
type OrphanedVolume struct {
	Name      string
	Path      string
	Size      int64
	ModTime   time.Time
	Container *database.Container // Original container info if available
}

// ScanOrphaned finds volumes on disk that don't have an active container
func ScanOrphaned() ([]*OrphanedVolume, error) {
	volumesDir := config.VolumesDir

	// Check if volumes directory exists
	if _, err := os.Stat(volumesDir); os.IsNotExist(err) {
		return []*OrphanedVolume{}, nil
	}

	// Get all active containers
	activeContainers, err := database.ListContainers()
	if err != nil {
		return nil, fmt.Errorf("failed to list active containers: %w", err)
	}

	// Build map of active volume names
	activeVolumes := make(map[string]*database.Container)
	for _, c := range activeContainers {
		if c.VolumeType == "named" && c.VolumePath != "" {
			activeVolumes[c.VolumePath] = c
		}
	}

	// Also get all containers (including expired) to find original container info
	allContainers, err := database.ListAllContainers()
	if err != nil {
		return nil, fmt.Errorf("failed to list all containers: %w", err)
	}

	allVolumes := make(map[string]*database.Container)
	for _, c := range allContainers {
		if c.VolumeType == "named" && c.VolumePath != "" {
			allVolumes[c.VolumePath] = c
		}
	}

	// Scan volumes directory
	entries, err := os.ReadDir(volumesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read volumes directory: %w", err)
	}

	var orphaned []*OrphanedVolume
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		volumeName := entry.Name()

		// Skip if this volume is actively used
		if _, active := activeVolumes[volumeName]; active {
			continue
		}

		// Get volume info
		volumePath := filepath.Join(volumesDir, volumeName)
		info, err := entry.Info()
		if err != nil {
			config.Logger.Warn("Failed to get info for volume", "volume", volumeName, "error", err)
			continue
		}

		// Calculate directory size
		size, err := getDirSize(volumePath)
		if err != nil {
			config.Logger.Warn("Failed to calculate size for volume", "volume", volumeName, "error", err)
			size = 0
		}

		orphan := &OrphanedVolume{
			Name:    volumeName,
			Path:    volumePath,
			Size:    size,
			ModTime: info.ModTime(),
		}

		// Try to find original container info
		if container, ok := allVolumes[volumeName]; ok {
			orphan.Container = container
		}

		orphaned = append(orphaned, orphan)
	}

	return orphaned, nil
}

// getDirSize calculates the total size of a directory
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// FormatSize formats bytes into human-readable format
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
