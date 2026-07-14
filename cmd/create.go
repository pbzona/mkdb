package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pbzona/mkdb/internal/adapters"
	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/credentials"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/types"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var (
	createName        string
	createVersion     string
	createPort        string
	createVolume      string
	createTTL         int
	createRepeat      bool
	createNoAuth      bool
	createNoWait      bool
	createWaitTimeout int
	createInit        string
)

var createCmd = &cobra.Command{
	Use:   "create [type]",
	Short: "Create a new database container",
	Long: `Create and start a new database container.

The database type (postgres, mysql, redis) may be given as an argument:
  mkdb create postgres --name myapp

With no arguments, mkdb prompts for the missing details interactively.
Databases are authenticated by default; pass --no-auth to disable.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&createName, "name", "", "Database name")
	createCmd.Flags().StringVar(&createVersion, "version", "", "Database version (default: latest stable)")
	createCmd.Flags().StringVar(&createPort, "port", "", "Host port to bind to")
	createCmd.Flags().StringVar(&createVolume, "volume", "", "Volume: 'none', 'named', or a host path for a bind mount")
	createCmd.Flags().IntVar(&createTTL, "ttl", 2, "Time to live in hours")
	createCmd.Flags().BoolVar(&createRepeat, "repeat", false, "Reuse the settings from the last database created")
	createCmd.Flags().BoolVar(&createNoAuth, "no-auth", false, "Create the database without authentication")
	createCmd.Flags().BoolVar(&createNoWait, "no-wait", false, "Return immediately without waiting for the database to accept connections")
	createCmd.Flags().IntVar(&createWaitTimeout, "wait-timeout", 30, "Seconds to wait for the database to become ready")
	createCmd.Flags().StringVar(&createInit, "init", "", "Path to a SQL script to run once the database is ready (postgres/mysql)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	settings, err := buildSettings(cmd, args)
	if err != nil {
		return err
	}

	normalizedType, err := types.NormalizeDBType(settings.DBType)
	if err != nil {
		return err
	}
	settings.DBType = normalizedType

	// Validate --init up front so we fail before creating any container.
	if createInit != "" {
		if normalizedType == types.DBTypeRedis {
			return fmt.Errorf("--init is not supported for redis")
		}
		if _, err := os.Stat(createInit); err != nil {
			return fmt.Errorf("init script not found: %s", createInit)
		}
	}

	dbConfig, err := docker.GetDBConfig(settings.DBType, settings.Version)
	if err != nil {
		return err
	}

	// Record the resolved version (adapter default when unspecified).
	if settings.Version == "" {
		if parts := strings.SplitN(dbConfig.Image, ":", 2); len(parts) == 2 {
			settings.Version = parts[1]
		}
	}

	containerName := "mkdb-" + settings.Name
	if _, err := database.GetContainer(containerName); err == nil {
		return fmt.Errorf("a database named '%s' already exists", settings.Name)
	}

	portExplicit := cmd.Flags().Changed("port") && strings.TrimSpace(createPort) != ""
	hostPort, err := resolvePort(settings.Port, dbConfig.DefaultPort, portExplicit)
	if err != nil {
		return err
	}
	settings.Port = hostPort

	volumeType, volumePath, err := resolveVolume(settings)
	if err != nil {
		return err
	}

	authEnabled, err := resolveAuth(cmd)
	if err != nil {
		return err
	}

	var username, password string
	if authEnabled {
		username = credentials.DefaultUsername
		password, err = credentials.GeneratePassword(passwordLength)
		if err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}
	}

	ui.Info(fmt.Sprintf("Creating %s database '%s'...", settings.DBType, settings.Name))
	if !authEnabled {
		ui.Info("Authentication disabled")
	}

	containerID, hostPort, err := createContainerWithAvailablePort(
		settings.DBType, settings.Name, username, password,
		hostPort, volumeType, volumePath, settings.Version, portExplicit,
	)
	if err != nil {
		return err
	}
	settings.Port = hostPort

	now := time.Now()
	container := &database.Container{
		Name:        containerName,
		DisplayName: settings.Name,
		Type:        settings.DBType,
		Version:     settings.Version,
		ContainerID: containerID,
		Port:        hostPort,
		Status:      types.StatusRunning,
		CreatedAt:   now,
		ExpiresAt:   now.Add(time.Duration(settings.TTLHours) * time.Hour),
		VolumeType:  volumeType,
		VolumePath:  volumePath,
	}

	if err := database.CreateContainer(container); err != nil {
		removeErr := docker.RemoveContainer(containerID) // best-effort rollback
		if removeErr != nil && config.Logger != nil {
			config.Logger.Warn("Failed to roll back container", "name", settings.Name, "error", removeErr)
		}
		return fmt.Errorf("failed to store container: %w", err)
	}

	var passwordHash string
	if authEnabled {
		passwordHash, err = config.Encrypt(password)
		if err != nil {
			rollbackCreatedContainer(container)
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
	}

	if err := database.CreateUser(&database.User{
		ContainerID:  container.ID,
		Username:     username,
		PasswordHash: passwordHash,
		IsDefault:    true,
		CreatedAt:    now,
	}); err != nil {
		rollbackCreatedContainer(container)
		return fmt.Errorf("failed to create user: %w", err)
	}

	if err := config.SaveLastSettings(settings); err != nil {
		config.Logger.Warn("Failed to save last settings", "error", err)
	}

	rawURL := rawConnectionString(container, username, password)

	// emit writes the primary result: JSON when requested, otherwise the
	// connection string on stdout plus human status on stderr. failed
	// suppresses the success chrome so the returned error is not contradicted.
	emit := func(ready *bool, failed bool) error {
		if jsonOutput {
			return outputJSON(containerToJSON(container, rawURL, ready))
		}
		if !failed {
			ui.Success(fmt.Sprintf("Database '%s' created", settings.Name))
		}
		fmt.Println(credentials.FormatEnvVar(rawURL))
		if !failed {
			ui.Info(fmt.Sprintf("Expires in %s (at %s)",
				humanizeHours(settings.TTLHours),
				container.ExpiresAt.Format("2006-01-02 15:04:05")))
			ui.Info("Reuse these settings with 'mkdb create --repeat'")
		}
		return nil
	}

	// Wait for the database to accept connections. A schema init always needs a
	// ready database, so it forces a wait even under --no-wait.
	if !createNoWait || createInit != "" {
		if !jsonOutput {
			ui.Info("Waiting for the database to become ready...")
		}
		waitErr := waitForReady(container, username, password, time.Duration(createWaitTimeout)*time.Second)
		ready := waitErr == nil
		if waitErr != nil {
			_ = emit(&ready, true)
			return withExitCode(exitTimeout, waitErr)
		}

		if createInit != "" {
			if err := runInitScript(container, username, password, createInit); err != nil {
				_ = emit(&ready, true)
				return err
			}
			if !jsonOutput {
				ui.Success(fmt.Sprintf("Applied schema from %s", createInit))
			}
		}

		return emit(&ready, false)
	}

	return emit(nil, false)
}

// runInitScript reads a SQL script and applies it to the container's database
// using the engine's stdin-driven client.
func runInitScript(container *database.Container, adminUser, adminPassword, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read init script: %w", err)
	}

	adapter, err := adapters.GetRegistry().Get(container.Type)
	if err != nil {
		return err
	}
	cmd := adapter.InitCommand(adminUser, adminPassword, container.DisplayName)
	if cmd == nil {
		return fmt.Errorf("--init is not supported for %s", container.Type)
	}

	if _, err := docker.ExecCommandStdin(container.ContainerID, cmd, data); err != nil {
		return fmt.Errorf("failed to apply init script: %w", err)
	}
	return nil
}

// buildSettings assembles creation settings from --repeat, flags, and prompts.
func buildSettings(cmd *cobra.Command, args []string) (*config.LastSettings, error) {
	if createRepeat {
		last, err := config.LoadLastSettings()
		if err != nil {
			return nil, fmt.Errorf("failed to load last settings: %w", err)
		}
		if last == nil {
			return nil, fmt.Errorf("no previous settings found; create a database first")
		}
		ui.Info(fmt.Sprintf("Reusing settings: %s database '%s'", last.DBType, last.Name))
		if cmd.Flags().Changed("port") {
			last.Port = createPort
		}
		if last.TTLHours == 0 {
			last.TTLHours = createTTL
		}
		return last, nil
	}

	var dbType string
	if len(args) > 0 {
		dbType = args[0]
	}

	settings := &config.LastSettings{
		DBType:     dbType,
		Name:       createName,
		Version:    createVersion,
		Port:       createPort,
		VolumePath: createVolume,
		TTLHours:   createTTL,
	}

	if settings.DBType == "" {
		selected, err := ui.SelectDBType()
		if err != nil {
			return nil, err
		}
		settings.DBType = selected
	}

	if settings.Name == "" {
		name, err := ui.PromptString("Enter database name", "")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("database name is required (pass --name)")
		}
		settings.Name = strings.TrimSpace(name)
	}

	return settings, nil
}

// resolvePort selects the host port, auto-advancing from the preferred port
// when it is not explicitly requested and is unavailable.
func resolvePort(requested, defaultPort string, explicit bool) (string, error) {
	requested = strings.TrimSpace(requested)
	if explicit {
		available, err := docker.IsPortAvailable(requested)
		if err != nil {
			return "", err
		}
		if !available {
			return "", fmt.Errorf("port %s is already in use", requested)
		}
		return requested, nil
	}

	preferred := defaultPort
	if requested != "" {
		preferred = requested
	}
	available, err := docker.IsPortAvailable(preferred)
	if err != nil {
		return "", err
	}
	if available {
		return preferred, nil
	}

	ui.Warning(fmt.Sprintf("Port %s is in use, finding the next available port...", preferred))
	port, err := docker.FindAvailablePort(preferred)
	if err != nil {
		return "", err
	}
	ui.Info(fmt.Sprintf("Using port %s", port))
	return port, nil
}

const maxAutomaticPortRetries = 100

// createContainerWithAvailablePort retries Docker-level port conflicts. A
// preflight socket check cannot eliminate the race between checking a port and
// asking Docker to publish it, especially when Docker Desktop owns the socket.
func createContainerWithAvailablePort(dbType, displayName, username, password, port, volumeType, volumePath, version string, explicitPort bool) (string, string, error) {
	return createContainerWithPort(port, explicitPort, func(port string) (string, error) {
		return docker.CreateContainer(
			dbType, displayName, username, password,
			port, volumeType, volumePath, version,
		)
	})
}

func createContainerWithPort(port string, explicitPort bool, create func(string) (string, error)) (string, string, error) {
	for attempt := 0; attempt < maxAutomaticPortRetries; attempt++ {
		containerID, err := create(port)
		if err == nil {
			return containerID, port, nil
		}

		if !docker.IsPortConflict(err) {
			return "", port, fmt.Errorf("failed to create container: %w", err)
		}
		if explicitPort {
			return "", port, fmt.Errorf("port %s is already in use", port)
		}

		next, err := nextPort(port)
		if err != nil {
			return "", port, err
		}
		next, err = docker.FindAvailablePort(next)
		if err != nil {
			return "", port, fmt.Errorf("port %s is unavailable and no alternative port was found: %w", port, err)
		}
		ui.Warning(fmt.Sprintf("Port %s became unavailable, trying port %s...", port, next))
		port = next
	}

	return "", port, fmt.Errorf("unable to find an available host port after %d attempts", maxAutomaticPortRetries)
}

func nextPort(port string) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(port))
	if err != nil {
		return "", fmt.Errorf("invalid port %q", port)
	}
	if n >= 65535 {
		return "", fmt.Errorf("no available port after %s", port)
	}
	return strconv.Itoa(n + 1), nil
}

func rollbackCreatedContainer(container *database.Container) {
	if container.ContainerID != "" {
		if err := docker.RemoveContainer(container.ContainerID); err != nil && config.Logger != nil {
			config.Logger.Warn("Failed to roll back container", "name", container.DisplayName, "error", err)
		}
	}
	if container.ID != 0 {
		if err := database.DeleteContainer(container.ID); err != nil && config.Logger != nil {
			config.Logger.Warn("Failed to roll back container record", "name", container.DisplayName, "error", err)
		}
	}
}

// resolveVolume determines the volume type/path from flags, repeat settings, or
// an interactive prompt, creating any needed directories.
func resolveVolume(settings *config.LastSettings) (volumeType, volumePath string, err error) {
	choice := settings.VolumePath

	// Repeat settings carry an explicit type; honor it.
	if choice == "" && settings.VolumeType != "" {
		choice = settings.VolumeType
	}

	if choice == "" {
		selected, err := ui.SelectVolumeOption()
		if err != nil {
			return "", "", err
		}
		choice = selected
	}

	switch choice {
	case types.VolumeTypeNone, "":
		settings.VolumeType, settings.VolumePath = types.VolumeTypeNone, ""
		return types.VolumeTypeNone, "", nil

	case types.VolumeTypeNamed:
		dir := filepath.Join(config.VolumesDir, settings.Name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", "", fmt.Errorf("failed to create volume directory: %w", err)
		}
		settings.VolumeType, settings.VolumePath = types.VolumeTypeNamed, settings.Name
		return types.VolumeTypeNamed, settings.Name, nil

	case types.VolumeTypeCustom:
		path, err := ui.PromptString("Enter volume path", "")
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(path) == "" {
			return "", "", fmt.Errorf("volume path is required")
		}
		return bindVolume(settings, strings.TrimSpace(path))

	default:
		// Treat any other value as a host path (bind mount).
		return bindVolume(settings, choice)
	}
}

func bindVolume(settings *config.LastSettings, path string) (string, string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", "", fmt.Errorf("failed to create volume directory: %w", err)
		}
	}
	settings.VolumeType, settings.VolumePath = types.VolumeTypeBind, path
	return types.VolumeTypeBind, path, nil
}

// resolveAuth decides whether authentication is enabled. Non-interactively it
// defaults to on; --no-auth disables it.
func resolveAuth(cmd *cobra.Command) (bool, error) {
	if cmd.Flags().Changed("no-auth") {
		return !createNoAuth, nil
	}
	// Flag not given: prompt when interactive, otherwise default to enabled.
	return ui.PromptConfirm("Enable authentication? (recommended)", true)
}

func humanizeHours(hours int) string {
	if hours == 1 {
		return "1 hour"
	}
	return fmt.Sprintf("%d hours", hours)
}
