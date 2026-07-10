package cmd

import (
	"errors"
	"fmt"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var (
	rmName string
	rmYes  bool
)

var rmCmd = &cobra.Command{
	Use:     "rm [name]",
	Aliases: []string{"remove"},
	Short:   "Delete a database container and its volume",
	Long:    `Delete a database container along with its data volume. This cannot be undone.`,
	Args:    cobra.MaximumNArgs(1),
	RunE:    runRm,
}

func init() {
	rootCmd.AddCommand(rmCmd)
	rmCmd.Flags().StringVar(&rmName, "name", "", "Container name (skips interactive selection)")
	rmCmd.Flags().BoolVarP(&rmYes, "yes", "y", false, "Skip the confirmation prompt")
}

func runRm(cmd *cobra.Command, args []string) error {
	container, err := resolveContainer(args, rmName, "Select container to remove", nil)
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

	if !rmYes {
		if !ui.IsInteractive() {
			return fmt.Errorf("refusing to remove '%s' without confirmation; pass --yes", container.DisplayName)
		}
		confirmed, err := ui.PromptConfirm(
			fmt.Sprintf("Delete '%s' and its volume? This cannot be undone", container.DisplayName), false)
		if errors.Is(err, ui.ErrCancelled) {
			return nil
		}
		if err != nil {
			return err
		}
		if !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.Info(fmt.Sprintf("Removing container '%s'...", container.DisplayName))

	if container.ContainerID != "" && docker.ContainerExists(container.ContainerID) {
		if err := docker.StopContainer(container.ContainerID); err != nil {
			ui.Warning(fmt.Sprintf("Failed to stop container: %v", err))
		}
		if err := docker.RemoveContainer(container.ContainerID); err != nil {
			ui.Warning(fmt.Sprintf("Failed to remove container: %v", err))
		}
	}

	if err := docker.RemoveVolume(container.VolumeType, container.VolumePath); err != nil {
		ui.Warning(fmt.Sprintf("Failed to remove volume: %v", err))
	}

	if err := database.DeleteContainer(container.ID); err != nil {
		return fmt.Errorf("failed to delete container record: %w", err)
	}

	ui.Success(fmt.Sprintf("Container '%s' removed", container.DisplayName))
	return nil
}
