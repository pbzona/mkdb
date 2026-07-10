package cmd

import (
	"errors"
	"fmt"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/credentials"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/types"
	"github.com/pbzona/mkdb/internal/ui"
)

// passwordLength is the length of generated database passwords.
const passwordLength = 20

// errNoContainers is a sentinel returned by resolveContainer when there are no
// candidate containers to act on.
var errNoContainers = errors.New("no containers found")

// resolveContainer selects a container from (in order): the first positional
// argument, the --name flag, or an interactive picker over the candidates that
// pass filter (nil means all). It returns errNoContainers when nothing matches.
func resolveContainer(args []string, nameFlag, prompt string, filter func(*database.Container) bool) (*database.Container, error) {
	name := nameFlag
	if len(args) > 0 && args[0] != "" {
		name = args[0]
	}

	if name != "" {
		c, err := database.GetContainerByDisplayName(name)
		if err != nil {
			return nil, fmt.Errorf("container '%s' not found", name)
		}
		return c, nil
	}

	containers, err := database.ListContainers()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	if filter != nil {
		containers = filterContainers(containers, filter)
	}
	if len(containers) == 0 {
		return nil, errNoContainers
	}

	return ui.SelectContainer(containers, prompt)
}

func filterContainers(containers []*database.Container, filter func(*database.Container) bool) []*database.Container {
	var out []*database.Container
	for _, c := range containers {
		if filter(c) {
			out = append(out, c)
		}
	}
	return out
}

// isRunning reports whether a container is in the running state.
func isRunning(c *database.Container) bool {
	return c.Status == types.StatusRunning
}

// adminCreds returns the privileged credentials for administrative operations
// (user management, password rotation) derived from the container's default
// user. Both values are empty for unauthenticated databases.
func adminCreds(container *database.Container) (user, password string, err error) {
	u, err := database.GetDefaultUser(container.ID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get default user: %w", err)
	}
	if u.Username == "" || u.PasswordHash == "" {
		return "", "", nil
	}
	pw, err := config.Decrypt(u.PasswordHash)
	if err != nil {
		return "", "", fmt.Errorf("failed to decrypt password: %w", err)
	}
	return u.Username, pw, nil
}

// defaultConnectionString builds the DB_URL for a container's default user.
func defaultConnectionString(container *database.Container) (string, error) {
	user, err := database.GetDefaultUser(container.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get default user: %w", err)
	}

	username, password := "", ""
	if user.Username != "" && user.PasswordHash != "" {
		username = user.Username
		password, err = config.Decrypt(user.PasswordHash)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt password: %w", err)
		}
	}

	return connectionString(container, username, password), nil
}

// connectionString formats a DB_URL-style connection string for the container.
// Redis uses database index 0 as the identifier; other engines use the name.
func connectionString(container *database.Container, username, password string) string {
	identifier := container.DisplayName
	if container.Type == types.DBTypeRedis {
		identifier = "0"
	}
	connStr := credentials.FormatConnectionString(
		container.Type,
		username,
		password,
		"localhost",
		container.Port,
		identifier,
	)
	return credentials.FormatEnvVar(connStr)
}
