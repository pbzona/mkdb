package cmd

import (
	"fmt"
	"time"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop an existing database container",
	Long:  `Stop and remove an existing database container, but preserve the volume.`,
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	// Get all containers
	containers, err := database.ListContainers()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Filter running containers
	var running []*database.Container
	for _, c := range containers {
		if c.Status == "running" {
			running = append(running, c)
		}
	}

	if len(running) == 0 {
		ui.Warning("No running containers found")
		return nil
	}

	// Select container
	container, err := ui.SelectContainer(running, "Select container to stop")
	if err != nil {
		return fmt.Errorf("failed to select container: %w", err)
	}

	ui.Info(fmt.Sprintf("Stopping container '%s'...", container.DisplayName))

	// Stop container
	if container.ContainerID != "" && docker.ContainerExists(container.ContainerID) {
		if err := docker.StopContainer(container.ContainerID); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}

		// Remove container
		if err := docker.RemoveContainer(container.ContainerID); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}

	// Update status
	container.Status = "stopped"
	if err := database.UpdateContainer(container); err != nil {
		return fmt.Errorf("failed to update container status: %w", err)
	}

	// Log event
	event := &database.Event{
		ContainerID: container.ID,
		EventType:   "stopped",
		Timestamp:   time.Now(),
		Details:     "Container stopped by user",
	}
	database.CreateEvent(event)

	ui.Success(fmt.Sprintf("Container '%s' stopped successfully!", container.DisplayName))
	return nil
}
