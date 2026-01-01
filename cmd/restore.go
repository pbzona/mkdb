package cmd

import (
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/credentials"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/types"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/pbzona/mkdb/internal/volumes"
	"github.com/spf13/cobra"
)

var (
	restoreType    string
	restoreVersion string
	restorePort    string
	restoreTTL     int
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore a database from an existing volume",
	Long:  `Recreate a database container using data from an existing volume.`,
	RunE:  runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().StringVar(&restoreType, "type", "", "Database type (postgres, mysql, redis)")
	restoreCmd.Flags().StringVar(&restoreVersion, "version", "latest", "Database version")
	restoreCmd.Flags().StringVar(&restorePort, "port", "", "Host port to bind to")
	restoreCmd.Flags().IntVar(&restoreTTL, "ttl", 2, "Time to live in hours")
}

func runRestore(cmd *cobra.Command, args []string) error {
	// Scan for orphaned volumes
	orphaned, err := volumes.ScanOrphaned()
	if err != nil {
		return fmt.Errorf("failed to scan volumes: %w", err)
	}

	if len(orphaned) == 0 {
		ui.Warning("No orphaned volumes found")
		ui.Info("Use 'mkdb list -a' to see all databases including removed ones")
		return nil
	}

	// Prompt user to select a volume to restore
	selectedVolume, err := promptSelectVolume(orphaned)
	if err != nil {
		return fmt.Errorf("failed to select volume: %w", err)
	}

	// Get database type
	dbType := restoreType
	if dbType == "" {
		if selectedVolume.Container != nil && selectedVolume.Container.Type != "" {
			dbType = selectedVolume.Container.Type
			ui.Info(fmt.Sprintf("Using database type from original container: %s", dbType))
		} else {
			// Prompt for type
			dbType, err = ui.SelectDBType()
			if err != nil {
				return fmt.Errorf("failed to select database type: %w", err)
			}
		}
	}

	// Validate database type
	normalizedType, err := types.NormalizeDBType(dbType)
	if err != nil {
		return err
	}
	dbType = normalizedType

	// Get version (use original or default)
	version := restoreVersion
	if version == "latest" && selectedVolume.Container != nil && selectedVolume.Container.Version != "" {
		version = selectedVolume.Container.Version
		ui.Info(fmt.Sprintf("Using version from original container: %s", version))
	}

	// Generate container name
	containerName := "mkdb-" + selectedVolume.Name

	// Check if container already exists
	if _, err := database.GetContainer(containerName); err == nil {
		return fmt.Errorf("container with name '%s' already exists", selectedVolume.Name)
	}

	// Determine port
	dbConfig := docker.GetDBConfig(dbType, version)
	hostPort := restorePort
	if hostPort == "" {
		hostPort = dbConfig.DefaultPort
		available, err := docker.IsPortAvailable(hostPort)
		if err != nil {
			return fmt.Errorf("failed to check port availability: %w", err)
		}
		if !available {
			ui.Warning(fmt.Sprintf("Default port %s is in use, finding next available port...", hostPort))
			hostPort, err = docker.FindAvailablePort(hostPort)
			if err != nil {
				return fmt.Errorf("failed to find available port: %w", err)
			}
			ui.Info(fmt.Sprintf("Using port %s", hostPort))
		}
	} else {
		available, err := docker.IsPortAvailable(hostPort)
		if err != nil {
			return fmt.Errorf("failed to check port availability: %w", err)
		}
		if !available {
			return fmt.Errorf("port %s is already in use", hostPort)
		}
	}

	ui.Info(fmt.Sprintf("Restoring %s database '%s' from volume...", dbType, selectedVolume.Name))

	// Create container with the existing volume
	volumePath := selectedVolume.Path
	containerID, err := docker.CreateContainer(
		dbType,
		selectedVolume.Name,
		credentials.DefaultUsername,
		credentials.DefaultPassword,
		hostPort,
		"bind", // Use bind mount for restore
		volumePath,
		"", // Use default version for restored containers
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Store in database
	now := time.Now()
	expiresAt := now.Add(time.Duration(restoreTTL) * time.Hour)

	newContainer := &database.Container{
		Name:        containerName,
		DisplayName: selectedVolume.Name,
		Type:        dbType,
		Version:     version,
		ContainerID: containerID,
		Port:        hostPort,
		Status:      "running",
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
		VolumeType:  "named",
		VolumePath:  selectedVolume.Name,
	}

	if err := database.CreateContainer(newContainer); err != nil {
		docker.RemoveContainer(containerID)
		return fmt.Errorf("failed to store container in database: %w", err)
	}

	// Create default user record
	passwordHash, err := config.Encrypt(credentials.DefaultPassword)
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}

	user := &database.User{
		ContainerID:  newContainer.ID,
		Username:     credentials.DefaultUsername,
		PasswordHash: passwordHash,
		IsDefault:    true,
		CreatedAt:    now,
	}

	if err := database.CreateUser(user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Log event
	event := &database.Event{
		ContainerID: newContainer.ID,
		EventType:   "restored",
		Timestamp:   now,
		Details:     fmt.Sprintf("Container restored from volume with %s:%s", dbType, version),
	}
	database.CreateEvent(event)

	ui.Success(fmt.Sprintf("Database '%s' restored successfully!", selectedVolume.Name))

	// Display connection string
	connStr := credentials.FormatConnectionString(
		dbType,
		credentials.DefaultUsername,
		credentials.DefaultPassword,
		"localhost",
		hostPort,
		selectedVolume.Name,
	)

	fmt.Println()
	fmt.Println(credentials.FormatEnvVar(connStr))
	fmt.Println()

	ttlMsg := fmt.Sprintf("Database will expire in %d hours (at %s)", restoreTTL, expiresAt.Format("2006-01-02 15:04:05"))
	if restoreTTL == 1 {
		ttlMsg = fmt.Sprintf("Database will expire in 1 hour (at %s)", expiresAt.Format("2006-01-02 15:04:05"))
	}
	ui.Info(ttlMsg)

	return nil
}

func promptSelectVolume(orphaned []*volumes.OrphanedVolume) (*volumes.OrphanedVolume, error) {
	if len(orphaned) == 1 {
		// Only one volume, ask for confirmation
		vol := orphaned[0]
		label := formatVolumeLabel(vol)

		var confirm bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Restore Volume").
					Description(label).
					Value(&confirm),
			),
		)

		if err := form.Run(); err != nil {
			return nil, err
		}

		if !confirm {
			return nil, fmt.Errorf("restore cancelled")
		}

		return vol, nil
	}

	// Multiple volumes, use select
	options := make([]huh.Option[*volumes.OrphanedVolume], len(orphaned))
	for i, vol := range orphaned {
		label := formatVolumeLabel(vol)
		options[i] = huh.NewOption(label, vol)
	}

	var selected *volumes.OrphanedVolume
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[*volumes.OrphanedVolume]().
				Title("Select Volume to Restore").
				Description("Choose a volume to restore").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	return selected, nil
}

func formatVolumeLabel(vol *volumes.OrphanedVolume) string {
	label := vol.Name

	if vol.Container != nil {
		label += fmt.Sprintf(" (%s", vol.Container.Type)
		if vol.Container.Version != "" && vol.Container.Version != "latest" {
			label += fmt.Sprintf(":%s", vol.Container.Version)
		}
		label += ")"
	}

	label += fmt.Sprintf(" - %s", volumes.FormatSize(vol.Size))

	return label
}
