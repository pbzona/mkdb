package cmd

import (
	"encoding/json"
	"os"
	"time"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/types"
)

// jsonOutput is set by the global --json flag. When true, commands that support
// it emit a single machine-readable JSON document on stdout and suppress the
// human-formatted output. Status/log messages still go to stderr.
var jsonOutput bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output machine-readable JSON on stdout")
}

// outputJSON encodes v as indented JSON to stdout with a trailing newline.
func outputJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// dbJSON is the stable JSON representation of a database container shared by
// create, ls, info, and creds. Fields are additive-only to preserve
// compatibility for scripts and agents.
type dbJSON struct {
	Name             string `json:"name"`
	Type             string `json:"type"`
	Version          string `json:"version"`
	Status           string `json:"status"`
	Host             string `json:"host"`
	Port             string `json:"port"`
	URL              string `json:"url,omitempty"`
	Ready            *bool  `json:"ready,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	VolumeType       string `json:"volume_type"`
	VolumePath       string `json:"volume_path,omitempty"`
	Size             string `json:"size,omitempty"`
}

// containerToJSON builds a dbJSON from a container. url may be empty when it is
// unavailable (e.g. credentials could not be resolved); ready may be nil when
// readiness was not evaluated.
func containerToJSON(c *database.Container, url string, ready *bool) dbJSON {
	volumeType := c.VolumeType
	if volumeType == "" {
		volumeType = types.VolumeTypeNone
	}
	return dbJSON{
		Name:             c.DisplayName,
		Type:             c.Type,
		Version:          c.Version,
		Status:           effectiveStatus(c),
		Host:             "localhost",
		Port:             c.Port,
		URL:              url,
		Ready:            ready,
		CreatedAt:        c.CreatedAt.Format(time.RFC3339),
		ExpiresAt:        c.ExpiresAt.Format(time.RFC3339),
		ExpiresInSeconds: int(time.Until(c.ExpiresAt).Seconds()),
		VolumeType:       volumeType,
		VolumePath:       c.VolumePath,
	}
}
