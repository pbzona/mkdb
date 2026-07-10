package types

import (
	"fmt"
	"strings"

	"github.com/pbzona/mkdb/internal/adapters"
)

// Database types
const (
	DBTypePostgres = "postgres"
	DBTypeMySQL    = "mysql"
	DBTypeRedis    = "redis"
)

// Volume types
const (
	VolumeTypeNone   = "none"
	VolumeTypeNamed  = "named"
	VolumeTypeBind   = "bind"
	VolumeTypeCustom = "custom path"
)

// Container statuses
const (
	StatusRunning = "running"
	StatusStopped = "stopped"
	StatusExpired = "expired"
	StatusRemoved = "removed"
)

var (
	// ValidStatuses is a list of all valid container statuses.
	ValidStatuses = []string{StatusRunning, StatusStopped, StatusExpired, StatusRemoved}

	// statusAliases maps common aliases to canonical statuses.
	statusAliases = map[string]string{
		"up":      StatusRunning,
		"running": StatusRunning,
		"down":    StatusStopped,
		"stopped": StatusStopped,
		"expired": StatusExpired,
		"removed": StatusRemoved,
	}
)

// ValidDBTypes returns a list of all valid database types from the adapter registry.
func ValidDBTypes() []string {
	registry := adapters.GetRegistry()
	return registry.List()
}

// NormalizeDBType normalizes a database type string to canonical form.
func NormalizeDBType(dbType string) (string, error) {
	registry := adapters.GetRegistry()
	canonical, err := registry.NormalizeType(dbType)
	if err != nil {
		return "", fmt.Errorf("invalid database type: %s (valid types: %s)", dbType, strings.Join(ValidDBTypes(), ", "))
	}
	return canonical, nil
}

// NormalizeStatus normalizes a status string (or alias) to canonical form.
func NormalizeStatus(status string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if canonical, ok := statusAliases[normalized]; ok {
		return canonical, nil
	}
	return "", fmt.Errorf("invalid status: %s (valid statuses: %s)", status, strings.Join(ValidStatuses, ", "))
}
