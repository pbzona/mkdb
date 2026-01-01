package cmd

import (
	"fmt"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var (
	infoContainerName string
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display container information",
	Long:  `Display detailed information about a database container including status, version, port, and TTL.`,
	RunE:  runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().StringVar(&infoContainerName, "name", "", "Container name (skips interactive selection)")
}

func runInfo(cmd *cobra.Command, args []string) error {
	var container *database.Container
	var err error

	// If name is provided, look it up directly
	if infoContainerName != "" {
		container, err = database.GetContainerByDisplayName(infoContainerName)
		if err != nil {
			return fmt.Errorf("container '%s' not found", infoContainerName)
		}
	} else {
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
		container, err = ui.SelectContainer(containers, "Select container to view")
		if err != nil {
			return fmt.Errorf("failed to select container: %w", err)
		}
	}

	// Try to get the actual version from the running container
	if container.Status == "running" && container.ContainerID != "" {
		actualVersion, err := docker.GetActualVersion(container.ContainerID, container.Type)
		if err == nil && actualVersion != "" {
			// Update the container version with the actual version
			container.Version = actualVersion
		}
		// If error, just use the stored version (tag like "latest")
	}

	// Print container info
	ui.PrintContainerInfo(container)

	return nil
}
