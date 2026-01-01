package cmd

import (
	"fmt"
	"time"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var extendHours int

var extendCmd = &cobra.Command{
	Use:   "extend",
	Short: "Extend the TTL of a container",
	Long:  `Extend the time-to-live of a database container to prevent automatic cleanup.`,
	RunE:  runExtend,
}

func init() {
	rootCmd.AddCommand(extendCmd)
	extendCmd.Flags().IntVar(&extendHours, "hours", 1, "Number of hours to extend TTL")
}

func runExtend(cmd *cobra.Command, args []string) error {
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
	container, err := ui.SelectContainer(containers, "Select container to extend TTL")
	if err != nil {
		return fmt.Errorf("failed to select container: %w", err)
	}

	// Extend TTL
	container.ExpiresAt = container.ExpiresAt.Add(time.Duration(extendHours) * time.Hour)

	if err := database.UpdateContainer(container); err != nil {
		return fmt.Errorf("failed to update container: %w", err)
	}

	// Log event
	event := &database.Event{
		ContainerID: container.ID,
		EventType:   "ttl_extended",
		Timestamp:   time.Now(),
		Details:     fmt.Sprintf("TTL extended by %d hours", extendHours),
	}
	database.CreateEvent(event)

	ui.Success(fmt.Sprintf("Container '%s' TTL extended by %d hours!", container.DisplayName, extendHours))
	ui.Info(fmt.Sprintf("New expiration: %s", container.ExpiresAt.Format("2006-01-02 15:04:05")))

	return nil
}
