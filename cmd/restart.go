package cmd

import (
	"fmt"
	"time"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart a database container",
	Long:  `Restart an existing database container.`,
	RunE:  runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
	// Get all containers
	containers, err := database.ListContainers()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		ui.Warning("No containers found")
		return nil
	}

	// Select container
	container, err := ui.SelectContainer(containers, "Select container to restart")
	if err != nil {
		return fmt.Errorf("failed to select container: %w", err)
	}

	ui.Info(fmt.Sprintf("Restarting container '%s'...", container.DisplayName))

	// Check if container exists
	if container.ContainerID != "" && docker.ContainerExists(container.ContainerID) {
		// Container exists, just restart it
		if err := docker.RestartContainer(container.ContainerID); err != nil {
			return fmt.Errorf("failed to restart container: %w", err)
		}
	} else {
		// Container doesn't exist, recreate it
		ui.Info("Container not found, recreating...")

		// Get default user credentials
		user, err := database.GetDefaultUser(container.ID)
		if err != nil {
			return fmt.Errorf("failed to get default user: %w", err)
		}

		password, err := config.Decrypt(user.PasswordHash)
		if err != nil {
			return fmt.Errorf("failed to decrypt password: %w", err)
		}

		containerID, err := docker.CreateContainer(
			container.Type,
			container.DisplayName,
			user.Username,
			password,
			container.Port,
			container.VolumeType,
			container.VolumePath,
		)
		if err != nil {
			return fmt.Errorf("failed to create container: %w", err)
		}

		container.ContainerID = containerID
	}

	// Update status
	container.Status = "running"
	if err := database.UpdateContainer(container); err != nil {
		return fmt.Errorf("failed to update container status: %w", err)
	}

	// Log event
	event := &database.Event{
		ContainerID: container.ID,
		EventType:   "restarted",
		Timestamp:   time.Now(),
		Details:     "Container restarted by user",
	}
	database.CreateEvent(event)

	ui.Success(fmt.Sprintf("Container '%s' restarted successfully!", container.DisplayName))
	return nil
}
