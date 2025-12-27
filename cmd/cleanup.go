package cmd

import (
	"fmt"

	"github.com/pbzona/mkdb/internal/cleanup"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up expired database containers",
	Long:  `Interactively select and remove expired database containers.`,
	RunE:  runCleanup,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	// Get expired containers
	containers, err := database.GetExpiredContainers()
	if err != nil {
		return fmt.Errorf("failed to get expired containers: %w", err)
	}

	if len(containers) == 0 {
		ui.Info("No expired containers found")
		return nil
	}

	ui.Info(fmt.Sprintf("Found %d expired container(s)", len(containers)))

	// Force cleanup to run (it will prompt for selection)
	return cleanup.RunInteractive(containers)
}
