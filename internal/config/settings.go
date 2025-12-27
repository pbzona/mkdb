package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const SettingsFileName = "last_settings.json"

// LastSettings stores the last used settings for quick repeat
type LastSettings struct {
	DBType     string `json:"db_type"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Port       string `json:"port"`
	VolumeType string `json:"volume_type"`
	VolumePath string `json:"volume_path"`
	TTLHours   int    `json:"ttl_hours"`
}

// SaveLastSettings saves settings to disk
func SaveLastSettings(settings *LastSettings) error {
	settingsPath := filepath.Join(DataDir, SettingsFileName)

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	return nil
}

// LoadLastSettings loads settings from disk
func LoadLastSettings() (*LastSettings, error) {
	settingsPath := filepath.Join(DataDir, SettingsFileName)

	// Check if file exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return nil, nil // No previous settings, not an error
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}

	var settings LastSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &settings, nil
}

// HasLastSettings checks if last settings exist
func HasLastSettings() bool {
	settingsPath := filepath.Join(DataDir, SettingsFileName)
	_, err := os.Stat(settingsPath)
	return err == nil
}
