package cmd

import (
	"fmt"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var (
	statContainerName string
)

var statCmd = &cobra.Command{
	Use:   "stat",
	Short: "See info about a specific database container",
	Long:  `Display detailed information about a database container including TTL.`,
	RunE:  runStat,
}

func init() {
	rootCmd.AddCommand(statCmd)
	statCmd.Flags().StringVar(&statContainerName, "name", "", "Container name (skips interactive selection)")
}

func runStat(cmd *cobra.Command, args []string) error {
	var container *database.Container
	var err error

	// If name is provided, look it up directly
	if statContainerName != "" {
		container, err = database.GetContainerByDisplayName(statContainerName)
		if err != nil {
			return fmt.Errorf("container '%s' not found", statContainerName)
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
