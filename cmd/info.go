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
	infoName string
	infoPing bool
)

var infoCmd = &cobra.Command{
	Use:     "info [name]",
	Aliases: []string{"stat"},
	Short:   "Show details about a database container",
	Long:    `Show detailed information about a database container including status, version, port, and TTL. Use --ping to also verify connectivity.`,
	Args:    cobra.MaximumNArgs(1),
	RunE:    runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().StringVar(&infoName, "name", "", "Container name (skips interactive selection)")
	infoCmd.Flags().BoolVar(&infoPing, "ping", false, "Test connectivity to the database")
}

func runInfo(cmd *cobra.Command, args []string) error {
	container, err := resolveContainer(args, infoName, "Select container to inspect", nil)
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

	// Detect the live version from a running container when possible.
	if isRunning(container) && container.ContainerID != "" {
		if actual, err := docker.GetActualVersion(container.ContainerID, container.Type); err == nil && actual != "" {
			container.Version = actual
		}
	}

	if jsonOutput {
		out := containerToJSON(container, containerURL(container), nil)
		if infoPing && isRunning(container) {
			ok := pingContainer(container) == nil
			out.Ready = &ok
		}
		return outputJSON(out)
	}

	ui.PrintContainerInfo(container)

	if infoPing {
		return pingContainer(container)
	}
	return nil
}

// pingContainer runs a lightweight connectivity check using stored credentials.
func pingContainer(container *database.Container) error {
	if !isRunning(container) {
		return fmt.Errorf("container '%s' is not running", container.DisplayName)
	}

	adminUser, adminPassword, err := adminCreds(container)
	if err != nil {
		return err
	}

	probe, err := connectivityProbe(container.Type, container.DisplayName, adminUser, adminPassword)
	if err != nil {
		return err
	}

	ui.Info(fmt.Sprintf("Testing connectivity to '%s'...", container.DisplayName))
	if _, err := docker.ExecCommand(container.ContainerID, probe); err != nil {
		ui.Error("Connection failed")
		return fmt.Errorf("connectivity test failed: %w", err)
	}

	ui.Success("Connection successful")
	return nil
}
