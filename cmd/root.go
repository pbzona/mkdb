package cmd

import (
	"fmt"
	"os"

	"github.com/pbzona/mkdb/internal/cleanup"
	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mkdb",
	Short: "mkdb - Easily manage local database containers",
	Long: `mkdb creates and manages disposable local Docker database containers
for development. It supports PostgreSQL, MySQL, and Redis.

Container lifecycle:
  create   Create a new database container
  stop     Stop a running container (data is preserved)
  start    Start a stopped container
  restart  Restart a container
  rm       Delete a container and its volume
  cleanup  Review and remove expired containers`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// version needs no config, database, or Docker access.
		if cmd.Name() == versionCmd.Name() {
			return nil
		}

		if err := config.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}
		if err := database.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		if err := docker.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize Docker client: %w", err)
		}

		// Surface expired containers non-interactively. The cleanup command
		// does its own reporting, so skip the notice there.
		if cmd.Name() != cleanupCmd.Name() {
			cleanup.Notify()
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if err := database.Close(); err != nil {
			config.Logger.Warn("Failed to close database", "error", err)
		}
		if err := docker.Close(); err != nil {
			config.Logger.Warn("Failed to close Docker client", "error", err)
		}
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
