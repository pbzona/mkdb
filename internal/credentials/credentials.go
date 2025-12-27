package credentials

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/pbzona/mkdb/internal/adapters"
)

const (
	DefaultUsername = "dbuser"
	DefaultPassword = "$uper$ecret"
	charset         = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// GeneratePassword generates a random alphanumeric password of the specified length
func GeneratePassword(length int) (string, error) {
	password := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := range password {
		randomIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random password: %w", err)
		}
		password[i] = charset[randomIndex.Int64()]
	}

	return string(password), nil
}

// FormatConnectionString formats a connection string based on database type
func FormatConnectionString(dbType, username, password, host, port, dbName string) string {
	registry := adapters.GetRegistry()
	adapter, err := registry.Get(dbType)
	if err != nil {
		// Fallback to empty string if adapter not found
		return ""
	}
	return adapter.FormatConnectionString(username, password, host, port, dbName)
}

// FormatEnvVar formats the connection string as an environment variable
func FormatEnvVar(connectionString string) string {
	return fmt.Sprintf("DB_URL=%s", connectionString)
}
