package cmd

import (
	"errors"
	"fmt"

	"github.com/pbzona/mkdb/internal/cleanup"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var (
	cleanupYes    bool
	cleanupDryRun bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Review and remove expired database containers",
	Long: `Review expired database containers.

By default this is interactive. For non-interactive use (scripts, agents):
  --dry-run   list expired containers without removing anything
  --yes       remove all expired containers without prompting`,
	Args: cobra.NoArgs,
	RunE: runCleanup,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().BoolVarP(&cleanupYes, "yes", "y", false, "Remove all expired containers without prompting")
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "List expired containers without removing them")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	containers, err := database.GetExpiredContainers()
	if err != nil {
		return fmt.Errorf("failed to get expired containers: %w", err)
	}

	// Dry run: report the expired set and stop.
	if cleanupDryRun {
		if jsonOutput {
			return outputJSON(expiredJSON(containers))
		}
		if len(containers) == 0 {
			ui.Info("No expired containers found")
			return nil
		}
		ui.Info(fmt.Sprintf("%d expired container(s) would be removed:", len(containers)))
		for _, c := range containers {
			ui.Info(fmt.Sprintf("  - %s (%s)", c.DisplayName, c.Type))
		}
		return nil
	}

	// Non-interactive removal.
	if cleanupYes {
		removed := make([]dbJSON, 0, len(containers))
		for _, c := range containers {
			if err := cleanup.Remove(c); err != nil {
				ui.Error(fmt.Sprintf("Failed to remove %s: %v", c.DisplayName, err))
				continue
			}
			if !jsonOutput {
				ui.Success(fmt.Sprintf("Removed %s (%s)", c.DisplayName, c.Type))
			}
			removed = append(removed, containerToJSON(c, "", nil))
		}
		if jsonOutput {
			return outputJSON(removed)
		}
		if len(containers) == 0 {
			ui.Info("No expired containers found")
		}
		return nil
	}

	// Interactive path.
	if len(containers) == 0 {
		ui.Info("No expired containers found")
		return nil
	}

	if !ui.IsInteractive() {
		return fmt.Errorf("no terminal available; use 'mkdb cleanup --yes' or 'mkdb cleanup --dry-run'")
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

// expiredJSON renders the expired container set for --dry-run --json.
func expiredJSON(containers []*database.Container) []dbJSON {
	out := make([]dbJSON, 0, len(containers))
	for _, c := range containers {
		out = append(out, containerToJSON(c, "", nil))
	}
	return out
}
