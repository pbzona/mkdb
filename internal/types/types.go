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
)

var (
	// ValidVolumeTypes is a list of all valid volume types
	ValidVolumeTypes = []string{VolumeTypeNone, VolumeTypeNamed, VolumeTypeCustom}

	// ValidStatuses is a list of all valid container statuses
	ValidStatuses = []string{StatusRunning, StatusStopped, StatusExpired}

	// StatusAliases maps common aliases to canonical statuses
	StatusAliases = map[string]string{
		"up":      StatusRunning,
		"running": StatusRunning,
		"down":    StatusStopped,
		"stopped": StatusStopped,
		"expired": StatusExpired,
	}
)

// ValidDBTypes returns a list of all valid database types from the adapter registry
func ValidDBTypes() []string {
	registry := adapters.GetRegistry()
	return registry.List()
}

// DBTypeAliases returns a map of all database type aliases from the adapter registry
func DBTypeAliases() map[string]string {
	registry := adapters.GetRegistry()
	return registry.GetAllAliases()
}

// NormalizeDBType normalizes a database type string to canonical form
func NormalizeDBType(dbType string) (string, error) {
	registry := adapters.GetRegistry()
	canonical, err := registry.NormalizeType(dbType)
	if err != nil {
		return "", fmt.Errorf("invalid database type: %s (valid types: %s)", dbType, strings.Join(ValidDBTypes(), ", "))
	}
	return canonical, nil
}

// NormalizeStatus normalizes a status string to canonical form
func NormalizeStatus(status string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if canonical, ok := StatusAliases[normalized]; ok {
		return canonical, nil
	}
	return "", fmt.Errorf("invalid status: %s (valid statuses: %s)", status, strings.Join(ValidStatuses, ", "))
}

// IsValidDBType checks if a database type is valid
func IsValidDBType(dbType string) bool {
	registry := adapters.GetRegistry()
	return registry.IsValidType(dbType)
}

// IsValidStatus checks if a status is valid
func IsValidStatus(status string) bool {
	_, err := NormalizeStatus(status)
	return err == nil
}
