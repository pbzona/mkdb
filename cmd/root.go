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
	Long: `mkdb is a CLI tool to create and manage local Docker database containers
for development environments. It supports PostgreSQL, MySQL, and Redis.`,
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize configuration
		if err := config.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		// Initialize database
		if err := database.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		// Initialize Docker client
		if err := docker.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize Docker client: %w", err)
		}

		// Run cleanup to check for expired containers
		if err := cleanup.Run(); err != nil {
			config.Logger.Warn("Cleanup failed", "error", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Close database connection
		if err := database.Close(); err != nil {
			config.Logger.Warn("Failed to close database", "error", err)
		}

		// Close Docker client
		if err := docker.Close(); err != nil {
			config.Logger.Warn("Failed to close Docker client", "error", err)
		}

		return nil
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
