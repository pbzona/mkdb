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

// RunInteractive prompts the user to select containers to extend or remove
func RunInteractive(containers []*database.Container) error {
	// First, prompt user to select containers to extend
	toExtend, extendHours, err := promptForExtend(containers)
	if err != nil {
		return fmt.Errorf("failed to prompt for extend: %w", err)
	}

	// Extend selected containers
	extendedCount := 0
	if len(toExtend) > 0 {
		for _, c := range toExtend {
			if err := extendContainer(c, extendHours); err != nil {
				config.Logger.Error("Failed to extend container", "name", c.DisplayName, "error", err)
				fmt.Printf("✗ Failed to extend %s: %v\n", c.DisplayName, err)
				continue
			}
			fmt.Printf("✓ Extended %s (%s) by %d hours\n", c.DisplayName, c.Type, extendHours)
			extendedCount++
		}
	}

	// Build list of containers not extended for removal prompt
	remainingContainers := make([]*database.Container, 0)
	for _, c := range containers {
		extended := false
		for _, e := range toExtend {
			if c.ID == e.ID {
				extended = true
				break
			}
		}
		if !extended {
			remainingContainers = append(remainingContainers, c)
		}
	}

	// Prompt user to select which containers to remove
	toRemove := []*database.Container{}
	if len(remainingContainers) > 0 {
		toRemove, err = promptForRemoval(remainingContainers)
		if err != nil {
			return fmt.Errorf("failed to prompt for removal: %w", err)
		}
	}

	// Clean up selected containers
	removedCount := 0
	for _, c := range toRemove {
		if err := cleanupContainer(c); err != nil {
			config.Logger.Error("Failed to cleanup container", "name", c.DisplayName, "error", err)
			fmt.Printf("✗ Failed to remove %s: %v\n", c.DisplayName, err)
			continue
		}
		fmt.Printf("✓ Removed %s (%s)\n", c.DisplayName, c.Type)
		removedCount++
	}

	// Print summary
	if extendedCount > 0 || removedCount > 0 {
		fmt.Println()
		if extendedCount > 0 {
			fmt.Printf("✓ Extended %d container(s)\n", extendedCount)
		}
		if removedCount > 0 {
			fmt.Printf("✓ Removed %d container(s)\n", removedCount)
		}
	} else {
		fmt.Println("\n✓ No changes made")
	}

	return nil
}

// promptForExtend shows an interactive prompt to select expired containers to extend
func promptForExtend(containers []*database.Container) ([]*database.Container, int, error) {
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
	var extendHoursStr string = "24" // Default to 24 hours

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
				Title("⏰ Extend Expired Databases").
				Description("Select databases to extend (Space to select, a=all, A=none, Enter to continue)").
				Options(options...).
				Value(&selected).
				WithKeyMap(keyMap),
		),
	)

	err := form.Run()
	if err != nil {
		return nil, 0, err
	}

	// If containers were selected, ask for hours
	extendHours := 24
	if len(selected) > 0 {
		hoursForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Extend by how many hours?").
					Value(&extendHoursStr).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("hours cannot be empty")
						}
						// Try to parse as int
						_, err := fmt.Sscanf(s, "%d", &extendHours)
						if err != nil {
							return fmt.Errorf("hours must be a valid number")
						}
						if extendHours <= 0 {
							return fmt.Errorf("hours must be greater than 0")
						}
						return nil
					}),
			),
		)

		err = hoursForm.Run()
		if err != nil {
			return nil, 0, err
		}

		// Parse the hours string to int
		fmt.Sscanf(extendHoursStr, "%d", &extendHours)
	}

	return selected, extendHours, nil
}

// promptForRemoval shows an interactive prompt to select expired containers to remove
func promptForRemoval(containers []*database.Container) ([]*database.Container, error) {
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
				Title("🗑️  Remove Expired Databases").
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

// extendContainer extends the TTL of a container, handling expired containers correctly
func extendContainer(c *database.Container, hours int) error {
	config.Logger.Info("Extending container TTL", "name", c.DisplayName, "hours", hours)

	// If container is already expired, extend from now instead of from old expiration time
	if time.Now().After(c.ExpiresAt) {
		config.Logger.Info("Container is expired, extending from current time", "name", c.DisplayName)
		c.ExpiresAt = time.Now().Add(time.Duration(hours) * time.Hour)
	} else {
		// Container is still valid, extend from current expiration
		c.ExpiresAt = c.ExpiresAt.Add(time.Duration(hours) * time.Hour)
	}

	// Update container in database
	if err := database.UpdateContainer(c); err != nil {
		return fmt.Errorf("failed to update container: %w", err)
	}

	// Log event
	event := &database.Event{
		ContainerID: c.ID,
		EventType:   "ttl_extended",
		Timestamp:   time.Now(),
		Details:     fmt.Sprintf("TTL extended by %d hours", hours),
	}
	if err := database.CreateEvent(event); err != nil {
		config.Logger.Warn("Failed to log event", "error", err)
	}

	config.Logger.Info("Container TTL extended", "name", c.DisplayName, "new_expiration", c.ExpiresAt)
	return nil
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
