package cmd

import (
	"errors"
	"fmt"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/types"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var stopName string

var stopCmd = &cobra.Command{
	Use:   "stop [name]",
	Short: "Stop a running database container",
	Long:  `Stop a running database container. The container and its data are preserved; use 'mkdb start' to run it again.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().StringVar(&stopName, "name", "", "Container name (skips interactive selection)")
}

func runStop(cmd *cobra.Command, args []string) error {
	container, err := resolveContainer(args, stopName, "Select container to stop", isRunning)
	if errors.Is(err, errNoContainers) {
		ui.Warning("No running containers found")
		return nil
	}
	if errors.Is(err, ui.ErrCancelled) {
		return nil
	}
	if err != nil {
		return err
	}

	if !isRunning(container) {
		return fmt.Errorf("container '%s' is not running", container.DisplayName)
	}

	ui.Info(fmt.Sprintf("Stopping container '%s'...", container.DisplayName))

	if container.ContainerID != "" && docker.ContainerExists(container.ContainerID) {
		if err := docker.StopContainer(container.ContainerID); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}
	}

	container.Status = types.StatusStopped
	if err := database.UpdateContainer(container); err != nil {
		return fmt.Errorf("failed to update container status: %w", err)
	}

	ui.Success(fmt.Sprintf("Container '%s' stopped", container.DisplayName))
	return nil
}
