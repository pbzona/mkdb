package cmd

import (
	"errors"
	"fmt"

	"github.com/pbzona/mkdb/internal/cleanup"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Review and remove expired database containers",
	Long:  `Interactively select expired database containers to extend or remove.`,
	Args:  cobra.NoArgs,
	RunE:  runCleanup,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	containers, err := database.GetExpiredContainers()
	if err != nil {
		return fmt.Errorf("failed to get expired containers: %w", err)
	}

	if len(containers) == 0 {
		ui.Info("No expired containers found")
		return nil
	}

	ui.Info(fmt.Sprintf("Found %d expired container(s)", len(containers)))

	if err := cleanup.RunInteractive(containers); err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			ui.Info("Cancelled")
			return nil
		}
		return err
	}
	return nil
}
