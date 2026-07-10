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

var restartName string

var restartCmd = &cobra.Command{
	Use:   "restart [name]",
	Short: "Restart a database container",
	Long:  `Restart a database container, recreating it from stored settings if the underlying container no longer exists.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.Flags().StringVar(&restartName, "name", "", "Container name (skips interactive selection)")
}

func runRestart(cmd *cobra.Command, args []string) error {
	container, err := resolveContainer(args, restartName, "Select container to restart", nil)
	if errors.Is(err, errNoContainers) {
		ui.Warning("No containers found")
		return nil
	}
	if errors.Is(err, ui.ErrCancelled) {
		return nil
	}
	if err != nil {
		return err
	}

	ui.Info(fmt.Sprintf("Restarting container '%s'...", container.DisplayName))

	if container.ContainerID != "" && docker.ContainerExists(container.ContainerID) {
		if err := docker.RestartContainer(container.ContainerID); err != nil {
			return fmt.Errorf("failed to restart container: %w", err)
		}
	} else if err := recreateContainer(container); err != nil {
		return err
	}

	container.Status = types.StatusRunning
	if err := database.UpdateContainer(container); err != nil {
		return fmt.Errorf("failed to update container status: %w", err)
	}

	ui.Success(fmt.Sprintf("Container '%s' restarted", container.DisplayName))
	return nil
}
