package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/credentials"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/types"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var (
	createName    string
	createVersion string
	createPort    string
	createVolume  string
	createTTL     int
	createRepeat  bool
	createNoAuth  bool
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

	hostPort, err := resolvePort(settings.Port, dbConfig.DefaultPort)
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

	containerID, err := docker.CreateContainer(
		settings.DBType, settings.Name, username, password,
		hostPort, volumeType, volumePath, settings.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

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
		docker.RemoveContainer(containerID) // best-effort rollback
		return fmt.Errorf("failed to store container: %w", err)
	}

	var passwordHash string
	if authEnabled {
		passwordHash, err = config.Encrypt(password)
		if err != nil {
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
		return fmt.Errorf("failed to create user: %w", err)
	}

	if err := config.SaveLastSettings(settings); err != nil {
		config.Logger.Warn("Failed to save last settings", "error", err)
	}

	ui.Success(fmt.Sprintf("Database '%s' created", settings.Name))

	// The connection string goes to stdout so it can be piped or eval'd.
	fmt.Println(connectionString(container, username, password))

	ui.Info(fmt.Sprintf("Expires in %s (at %s)",
		humanizeHours(settings.TTLHours),
		container.ExpiresAt.Format("2006-01-02 15:04:05")))
	ui.Info("Reuse these settings with 'mkdb create --repeat'")

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

// resolvePort selects the host port, auto-advancing from the default when free.
func resolvePort(requested, defaultPort string) (string, error) {
	if requested != "" {
		available, err := docker.IsPortAvailable(requested)
		if err != nil {
			return "", err
		}
		if !available {
			return "", fmt.Errorf("port %s is already in use", requested)
		}
		return requested, nil
	}

	available, err := docker.IsPortAvailable(defaultPort)
	if err != nil {
		return "", err
	}
	if available {
		return defaultPort, nil
	}

	ui.Warning(fmt.Sprintf("Port %s is in use, finding the next available port...", defaultPort))
	port, err := docker.FindAvailablePort(defaultPort)
	if err != nil {
		return "", err
	}
	ui.Info(fmt.Sprintf("Using port %s", port))
	return port, nil
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
