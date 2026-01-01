package cleanup

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
)

// Run checks for and cleans up expired containers
func Run() error {
	containers, err := database.GetExpiredContainers()
	if err != nil {
		return fmt.Errorf("failed to get expired containers: %w", err)
	}

	if len(containers) == 0 {
		return nil
	}

	config.Logger.Info("Found expired containers", "count", len(containers))

	// Check if we're in an interactive terminal
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		config.Logger.Info("Non-interactive terminal detected, skipping cleanup prompt")
		return nil
	}

	return RunInteractive(containers)
}

// RunInteractive prompts the user to select and clean up containers
func RunInteractive(containers []*database.Container) error {
	// Prompt user to select which containers to remove
	selected, err := promptForCleanup(containers)
	if err != nil {
		return fmt.Errorf("failed to prompt for cleanup: %w", err)
	}

	if len(selected) == 0 {
		config.Logger.Info("No containers selected for cleanup")
		fmt.Println("\n✓ No containers were removed")
		return nil
	}

	// Clean up selected containers
	successCount := 0
	for _, c := range selected {
		if err := cleanupContainer(c); err != nil {
			config.Logger.Error("Failed to cleanup container", "name", c.DisplayName, "error", err)
			fmt.Printf("✗ Failed to remove %s: %v\n", c.DisplayName, err)
			continue
		}
		fmt.Printf("✓ Removed %s (%s)\n", c.DisplayName, c.Type)
		successCount++
	}

	if successCount > 0 {
		fmt.Printf("\n✓ Successfully removed %d container(s)\n", successCount)
	}

	return nil
}

// promptForCleanup shows an interactive prompt to select expired containers to remove
func promptForCleanup(containers []*database.Container) ([]*database.Container, error) {
	// Build options for multiselect
	options := make([]huh.Option[*database.Container], len(containers))
	for i, c := range containers {
		// Calculate time since expiration
		expired := time.Since(c.ExpiresAt)
		expiredStr := formatExpiredDuration(expired)

		label := fmt.Sprintf("%s (%s) - expired %s ago", c.DisplayName, c.Type, expiredStr)
		options[i] = huh.NewOption(label, c)
	}

	var selected []*database.Container

	// Customize key bindings to use 'a' instead of 'ctrl+a' for select all
	keyMap := huh.NewDefaultKeyMap()
	keyMap.MultiSelect.SelectAll = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "select all"),
	)
	keyMap.MultiSelect.SelectNone = key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "select none"),
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[*database.Container]().
				Title("🗑️  Expired Databases").
				Description("Select databases to remove (Space to select, a=all, A=none, Enter to confirm)").
				Options(options...).
				Value(&selected).
				WithKeyMap(keyMap),
		),
	)

	err := form.Run()
	if err != nil {
		return nil, err
	}

	return selected, nil
}

// formatExpiredDuration formats how long ago a container expired
func formatExpiredDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

func cleanupContainer(c *database.Container) error {
	config.Logger.Info("Cleaning up expired container", "name", c.DisplayName)

	// Stop the container if it exists
	if c.ContainerID != "" && docker.ContainerExists(c.ContainerID) {
		if err := docker.StopContainer(c.ContainerID); err != nil {
			config.Logger.Warn("Failed to stop container", "name", c.DisplayName, "error", err)
		}

		// Remove the container
		if err := docker.RemoveContainer(c.ContainerID); err != nil {
			config.Logger.Warn("Failed to remove container", "name", c.DisplayName, "error", err)
		}
	}

	// Remove volume if it exists
	if c.VolumePath != "" {
		if err := docker.RemoveVolume(c.VolumePath); err != nil {
			config.Logger.Warn("Failed to remove volume", "name", c.DisplayName, "error", err)
		}
	}

	// Log the event before deleting from database
	event := &database.Event{
		ContainerID: c.ID,
		EventType:   "expired",
		Timestamp:   time.Now(),
		Details:     "Container automatically expired and cleaned up",
	}
	if err := database.CreateEvent(event); err != nil {
		config.Logger.Warn("Failed to log event", "error", err)
	}

	// Delete from database entirely instead of just marking as expired
	if err := database.DeleteContainer(c.ID); err != nil {
		return fmt.Errorf("failed to delete container from database: %w", err)
	}

	config.Logger.Info("Container cleanup complete", "name", c.DisplayName)
	return nil
}
