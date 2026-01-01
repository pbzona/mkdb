package cmd

import (
	"fmt"
	"time"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "Delete an existing container and its volume",
	Long:    `Delete an existing database container and its associated volume.`,
	RunE:    runRm,
}

func init() {
	rootCmd.AddCommand(rmCmd)
}

func runRm(cmd *cobra.Command, args []string) error {
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
	container, err := ui.SelectContainer(containers, "Select container to remove")
	if err != nil {
		return fmt.Errorf("failed to select container: %w", err)
	}

	// Confirm deletion
	confirmed, err := ui.PromptConfirm(fmt.Sprintf("Are you sure you want to delete '%s'? This will remove the container and its volume", container.DisplayName))
	if err != nil {
		return fmt.Errorf("failed to get confirmation: %w", err)
	}

	if !confirmed {
		ui.Info("Deletion cancelled")
		return nil
	}

	ui.Info(fmt.Sprintf("Removing container '%s'...", container.DisplayName))

	// Stop and remove container
	if container.ContainerID != "" && docker.ContainerExists(container.ContainerID) {
		if err := docker.StopContainer(container.ContainerID); err != nil {
			ui.Warning(fmt.Sprintf("Failed to stop container: %v", err))
		}

		if err := docker.RemoveContainer(container.ContainerID); err != nil {
			ui.Warning(fmt.Sprintf("Failed to remove container: %v", err))
		}
	}

	// Remove volume if it exists
	if container.VolumePath != "" {
		if err := docker.RemoveVolume(container.VolumePath); err != nil {
			ui.Warning(fmt.Sprintf("Failed to remove volume: %v", err))
		}
	}

	// Log event
	event := &database.Event{
		ContainerID: container.ID,
		EventType:   "deleted",
		Timestamp:   time.Now(),
		Details:     "Container deleted by user",
	}
	database.CreateEvent(event)

	// Delete from database
	if err := database.DeleteContainer(container.ID); err != nil {
		return fmt.Errorf("failed to delete container from database: %w", err)
	}

	ui.Success(fmt.Sprintf("Container '%s' removed successfully!", container.DisplayName))
	return nil
}
