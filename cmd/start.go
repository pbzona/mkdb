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
	dbType     string
	dbName     string
	version    string
	port       string
	volumeFlag string
	ttlHours   int
	useRepeat  bool
	noAuth     bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Create a new database container",
	Long:  `Create and start a new database container with persistent volume storage.`,
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVar(&dbType, "db", "", "Database type (postgres, redis, mysql)")
	startCmd.Flags().StringVar(&dbName, "name", "", "Database name")
	startCmd.Flags().StringVar(&version, "version", "", "Database version (default: latest)")
	startCmd.Flags().StringVar(&port, "port", "", "Host port to bind to")
	startCmd.Flags().StringVar(&volumeFlag, "volume", "", "Volume path (optional)")
	startCmd.Flags().IntVar(&ttlHours, "ttl", 2, "Time to live in hours")
	startCmd.Flags().BoolVar(&useRepeat, "repeat", false, "Use settings from last database created")
	startCmd.Flags().BoolVar(&noAuth, "no-auth", false, "Create database without authentication")
}

func runStart(cmd *cobra.Command, args []string) error {
	var settings *config.LastSettings

	// Check if using repeat mode
	if useRepeat {
		lastSettings, err := config.LoadLastSettings()
		if err != nil {
			return fmt.Errorf("failed to load last settings: %w", err)
		}
		if lastSettings == nil {
			return fmt.Errorf("no previous settings found, create a database first")
		}

		// Confirm with user
		ui.Info(fmt.Sprintf("Using previous settings: %s database '%s'", lastSettings.DBType, lastSettings.Name))
		confirmed, err := ui.PromptConfirm("Continue with these settings?")
		if err != nil {
			return err
		}
		if !confirmed {
			ui.Info("Cancelled")
			return nil
		}

		settings = lastSettings
	} else {
		// Build settings from flags and prompts
		settings = &config.LastSettings{
			DBType:     dbType,
			Name:       dbName,
			Version:    version,
			Port:       port,
			VolumePath: volumeFlag,
			TTLHours:   ttlHours,
		}

		// Prompt for missing required fields
		if err := promptForMissingFields(settings); err != nil {
			return err
		}
	}

	// Use TTL from settings, or default if not set
	if settings.TTLHours == 0 {
		settings.TTLHours = 2
	}

	// Validate database type
	normalizedType, err := types.NormalizeDBType(settings.DBType)
	if err != nil {
		return err
	}
	settings.DBType = normalizedType

	// Get database configuration
	dbConfig := docker.GetDBConfig(settings.DBType, settings.Version)

	// Store the actual version that will be used (adapter provides default if empty)
	if settings.Version == "" {
		// Get the actual version from the image string (e.g., "postgres:18" -> "18")
		imageParts := strings.Split(dbConfig.Image, ":")
		if len(imageParts) == 2 {
			settings.Version = imageParts[1]
		}
	}

	// Generate container name
	containerName := "mkdb-" + settings.Name

	// Check if container already exists
	if _, err := database.GetContainer(containerName); err == nil {
		return fmt.Errorf("container with name '%s' already exists", settings.Name)
	}

	// Determine port
	hostPort := settings.Port
	if hostPort == "" {
		// No port specified, use default and find next available if needed
		hostPort = dbConfig.DefaultPort
		available, err := docker.IsPortAvailable(hostPort)
		if err != nil {
			return fmt.Errorf("failed to check port availability: %w", err)
		}
		if !available {
			// Default port is taken, find next available
			ui.Warning(fmt.Sprintf("Default port %s is in use, finding next available port...", hostPort))
			hostPort, err = docker.FindAvailablePort(hostPort)
			if err != nil {
				return fmt.Errorf("failed to find available port: %w", err)
			}
			ui.Info(fmt.Sprintf("Using port %s", hostPort))
		}
	} else {
		// User specified a port, check if it's available
		available, err := docker.IsPortAvailable(hostPort)
		if err != nil {
			return fmt.Errorf("failed to check port availability: %w", err)
		}
		if !available {
			return fmt.Errorf("port %s is already in use (use default port for automatic selection)", hostPort)
		}
	}

	// Save the actual port used
	settings.Port = hostPort

	// Volume configuration
	var volumeType, volumePath string
	if settings.VolumePath != "" {
		// Volume path provided via flag
		// Check if it's a special value (none, named) or a path
		switch settings.VolumePath {
		case "none":
			volumeType = "none"
			volumePath = ""
			settings.VolumeType = volumeType
		case "named":
			volumeType = "named"
			volumePath = settings.Name
			settings.VolumeType = volumeType
			// Create the volume directory
			volumeDir := filepath.Join(config.VolumesDir, settings.Name)
			if err := os.MkdirAll(volumeDir, 0755); err != nil {
				return fmt.Errorf("failed to create volume directory: %w", err)
			}
		default:
			// Custom path
			volumeType = "bind"
			volumePath = settings.VolumePath
			settings.VolumeType = volumeType
			// Validate path
			if _, err := os.Stat(volumePath); os.IsNotExist(err) {
				if err := os.MkdirAll(volumePath, 0755); err != nil {
					return fmt.Errorf("failed to create volume directory: %w", err)
				}
			}
		}
	} else if settings.VolumeType != "" {
		// Volume type from repeat settings
		volumeType = settings.VolumeType
		volumePath = settings.VolumePath

		if volumeType == "named" && volumePath == "" {
			volumePath = settings.Name
			volumeDir := filepath.Join(config.VolumesDir, settings.Name)
			if err := os.MkdirAll(volumeDir, 0755); err != nil {
				return fmt.Errorf("failed to create volume directory: %w", err)
			}
		}
	} else {
		// Prompt for volume configuration
		volumeOption, err := ui.SelectVolumeOption()
		if err != nil {
			return fmt.Errorf("failed to select volume option: %w", err)
		}

		switch volumeOption {
		case "named":
			volumeType = "named"
			volumePath = settings.Name
			settings.VolumeType = volumeType
			settings.VolumePath = volumePath
			// Create the volume directory
			volumeDir := filepath.Join(config.VolumesDir, settings.Name)
			if err := os.MkdirAll(volumeDir, 0755); err != nil {
				return fmt.Errorf("failed to create volume directory: %w", err)
			}
		case "custom path":
			volumeType = "bind"
			volumePath, err = ui.PromptString("Enter volume path", "")
			if err != nil {
				return fmt.Errorf("failed to get volume path: %w", err)
			}
			settings.VolumeType = volumeType
			settings.VolumePath = volumePath
			// Validate path
			if _, err := os.Stat(volumePath); os.IsNotExist(err) {
				if err := os.MkdirAll(volumePath, 0755); err != nil {
					return fmt.Errorf("failed to create volume directory: %w", err)
				}
			}
		default:
			settings.VolumeType = "none"
			settings.VolumePath = ""
		}
	}

	// Determine credentials based on --no-auth flag or prompt
	var username, password string

	// Check if --no-auth flag was explicitly set
	noAuthFlagSet := cmd.Flags().Changed("no-auth")

	if noAuthFlagSet && noAuth {
		// Flag explicitly set to true - no authentication
		username = ""
		password = ""
	} else if !noAuthFlagSet {
		// Flag not set, prompt user
		useAuth, err := ui.PromptConfirm("Enable authentication? (recommended)")
		if err != nil {
			return fmt.Errorf("failed to get authentication preference: %w", err)
		}
		if useAuth {
			// Generate random password
			username = credentials.DefaultUsername
			password, err = credentials.GeneratePassword(12)
			if err != nil {
				return fmt.Errorf("failed to generate password: %w", err)
			}
		} else {
			username = ""
			password = ""
		}
	} else {
		// Flag explicitly set to false - use authentication with random password
		username = credentials.DefaultUsername
		password, err = credentials.GeneratePassword(12)
		if err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}
	}

	ui.Info(fmt.Sprintf("Creating %s database '%s'...", settings.DBType, settings.Name))

	if username == "" && password == "" {
		ui.Info("Creating database without authentication")
	}

	// Create container
	containerID, err := docker.CreateContainer(
		settings.DBType,
		settings.Name,
		username,
		password,
		hostPort,
		volumeType,
		volumePath,
		settings.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Store in database
	now := time.Now()
	expiresAt := now.Add(time.Duration(settings.TTLHours) * time.Hour)

	container := &database.Container{
		Name:        containerName,
		DisplayName: settings.Name,
		Type:        settings.DBType,
		Version:     settings.Version,
		ContainerID: containerID,
		Port:        hostPort,
		Status:      "running",
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
		VolumeType:  volumeType,
		VolumePath:  volumePath,
	}

	if err := database.CreateContainer(container); err != nil {
		// Try to clean up the Docker container
		docker.RemoveContainer(containerID)
		return fmt.Errorf("failed to store container in database: %w", err)
	}

	// Create default user (or unauthenticated entry if no auth)
	var passwordHash string
	if !noAuth {
		passwordHash, err = config.Encrypt(password)
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
	}

	user := &database.User{
		ContainerID:  container.ID,
		Username:     username,
		PasswordHash: passwordHash,
		IsDefault:    true,
		CreatedAt:    now,
	}

	if err := database.CreateUser(user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Log event
	event := &database.Event{
		ContainerID: container.ID,
		EventType:   "created",
		Timestamp:   now,
		Details:     fmt.Sprintf("Container created with %s:%s", settings.DBType, settings.Version),
	}
	database.CreateEvent(event)

	// Save settings for next time
	if err := config.SaveLastSettings(settings); err != nil {
		config.Logger.Warn("Failed to save last settings", "error", err)
	}

	ui.Success(fmt.Sprintf("Database '%s' created successfully!", settings.Name))

	// Display connection string
	connStr := credentials.FormatConnectionString(
		settings.DBType,
		username,
		password,
		"localhost",
		hostPort,
		settings.Name,
	)

	fmt.Println()
	fmt.Println(credentials.FormatEnvVar(connStr))
	fmt.Println()

	ttlMsg := fmt.Sprintf("Database will expire in %d hours (at %s)", settings.TTLHours, expiresAt.Format("2006-01-02 15:04:05"))
	if settings.TTLHours == 1 {
		ttlMsg = fmt.Sprintf("Database will expire in 1 hour (at %s)", expiresAt.Format("2006-01-02 15:04:05"))
	}
	ui.Info(ttlMsg)
	ui.Info("Use 'mkdb start --repeat' to quickly create another database with the same settings")

	return nil
}

func promptForMissingFields(settings *config.LastSettings) error {
	// Prompt for database type if not provided
	if settings.DBType == "" {
		// Check if we have last settings to offer as default
		lastSettings, _ := config.LoadLastSettings()
		if lastSettings != nil && config.HasLastSettings() {
			ui.Info(fmt.Sprintf("Last used: %s (press Enter to use, or select different type)", lastSettings.DBType))
		}

		dbType, err := ui.SelectDBType()
		if err != nil {
			return fmt.Errorf("failed to select database type: %w", err)
		}
		settings.DBType = dbType
	}

	// Prompt for database name if not provided
	if settings.Name == "" {
		name, err := ui.PromptString("Enter database name", "")
		if err != nil {
			return fmt.Errorf("failed to get database name: %w", err)
		}
		if name == "" {
			return fmt.Errorf("database name cannot be empty")
		}
		settings.Name = name
	}

	return nil
}
