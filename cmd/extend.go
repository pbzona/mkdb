package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var (
	extendHours int
	extendName  string
)

var extendCmd = &cobra.Command{
	Use:   "extend [name]",
	Short: "Extend the TTL of a container",
	Long:  `Extend the time-to-live of a database container to prevent automatic cleanup.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runExtend,
}

func init() {
	rootCmd.AddCommand(extendCmd)
	extendCmd.Flags().IntVar(&extendHours, "hours", 24, "Number of hours to extend the TTL by")
	extendCmd.Flags().StringVar(&extendName, "name", "", "Container name (skips interactive selection)")
}

func runExtend(cmd *cobra.Command, args []string) error {
	if extendHours <= 0 {
		return fmt.Errorf("--hours must be greater than 0")
	}

	container, err := resolveContainer(args, extendName, "Select container to extend", nil)
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

	// Reset from now if already expired, otherwise add to the current expiry.
	if time.Now().After(container.ExpiresAt) {
		ui.Info("Container has expired; extending from now")
		container.ExpiresAt = time.Now().Add(time.Duration(extendHours) * time.Hour)
	} else {
		container.ExpiresAt = container.ExpiresAt.Add(time.Duration(extendHours) * time.Hour)
	}

	if err := database.UpdateContainer(container); err != nil {
		return fmt.Errorf("failed to update container: %w", err)
	}

	ui.Success(fmt.Sprintf("Extended '%s' by %s", container.DisplayName, humanizeHours(extendHours)))
	ui.Info(fmt.Sprintf("New expiration: %s", container.ExpiresAt.Format("2006-01-02 15:04:05")))
	return nil
}
