package cmd

import (
	"errors"
	"fmt"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/types"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var startName string

var startCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start a stopped database container",
	Long:  `Start a previously stopped database container, recreating it from stored settings if the underlying container no longer exists.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVar(&startName, "name", "", "Container name (skips interactive selection)")
}

func runStart(cmd *cobra.Command, args []string) error {
	container, err := resolveContainer(args, startName, "Select container to start", func(c *database.Container) bool {
		return !isRunning(c)
	})
	if errors.Is(err, errNoContainers) {
		ui.Warning("No stopped containers found")
		return nil
	}
	if errors.Is(err, ui.ErrCancelled) {
		return nil
	}
	if err != nil {
		return err
	}

	if isRunning(container) {
		ui.Info(fmt.Sprintf("Container '%s' is already running", container.DisplayName))
		return nil
	}

	ui.Info(fmt.Sprintf("Starting container '%s'...", container.DisplayName))

	if container.ContainerID != "" && docker.ContainerExists(container.ContainerID) {
		if err := docker.StartContainer(container.ContainerID); err != nil {
			return fmt.Errorf("failed to start container: %w", err)
		}
	} else if err := recreateContainer(container); err != nil {
		return err
	}

	container.Status = types.StatusRunning
	if err := database.UpdateContainer(container); err != nil {
		return fmt.Errorf("failed to update container status: %w", err)
	}

	ui.Success(fmt.Sprintf("Container '%s' started", container.DisplayName))
	return nil
}

// recreateContainer rebuilds a container that no longer exists in Docker from
// its stored settings and credentials, updating container.ContainerID.
func recreateContainer(container *database.Container) error {
	ui.Info("Underlying container is missing, recreating...")

	user, err := database.GetDefaultUser(container.ID)
	if err != nil {
		return fmt.Errorf("failed to get default user: %w", err)
	}

	username, password := "", ""
	if user.Username != "" && user.PasswordHash != "" {
		username = user.Username
		password, err = config.Decrypt(user.PasswordHash)
		if err != nil {
			return fmt.Errorf("failed to decrypt password: %w", err)
		}
	}

	containerID, err := docker.CreateContainer(
		container.Type, container.DisplayName, username, password,
		container.Port, container.VolumeType, container.VolumePath, container.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to recreate container: %w", err)
	}

	container.ContainerID = containerID
	return nil
}
