package cleanup

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
)

// Notify prints a one-line, non-interactive warning when expired containers
// exist. It is safe to call before every command and never blocks or prompts.
func Notify() {
	containers, err := database.GetExpiredContainers()
	if err != nil || len(containers) == 0 {
		return
	}

	ui.Warning(fmt.Sprintf("%d database(s) expired — run 'mkdb cleanup' to review", len(containers)))
}

// RunInteractive prompts the user to select expired containers to extend or
// remove. It requires an interactive terminal.
func RunInteractive(containers []*database.Container) error {
	if !ui.IsInteractive() {
		return fmt.Errorf("cleanup requires an interactive terminal")
	}

	// Prompt for containers to extend.
	toExtend, extendHours, err := promptForExtend(containers)
	if err != nil {
		return err
	}

	extendedCount := 0
	for _, c := range toExtend {
		if err := extendContainer(c, extendHours); err != nil {
			ui.Error(fmt.Sprintf("Failed to extend %s: %v", c.DisplayName, err))
			continue
		}
		ui.Success(fmt.Sprintf("Extended %s (%s) by %d hours", c.DisplayName, c.Type, extendHours))
		extendedCount++
	}

	// Everything not extended is a removal candidate.
	remaining := make([]*database.Container, 0, len(containers))
	for _, c := range containers {
		if !contains(toExtend, c) {
			remaining = append(remaining, c)
		}
	}

	var toRemove []*database.Container
	if len(remaining) > 0 {
		toRemove, err = promptForRemoval(remaining)
		if err != nil {
			return err
		}
	}

	removedCount := 0
	for _, c := range toRemove {
		if err := cleanupContainer(c); err != nil {
			ui.Error(fmt.Sprintf("Failed to remove %s: %v", c.DisplayName, err))
			continue
		}
		ui.Success(fmt.Sprintf("Removed %s (%s)", c.DisplayName, c.Type))
		removedCount++
	}

	if extendedCount == 0 && removedCount == 0 {
		ui.Info("No changes made")
	}

	return nil
}

func contains(list []*database.Container, target *database.Container) bool {
	for _, c := range list {
		if c.ID == target.ID {
			return true
		}
	}
	return false
}

func containerOptions(containers []*database.Container) []huh.Option[*database.Container] {
	options := make([]huh.Option[*database.Container], len(containers))
	for i, c := range containers {
		expiredStr := formatExpiredDuration(time.Since(c.ExpiresAt))
		label := fmt.Sprintf("%s (%s) - expired %s ago", c.DisplayName, c.Type, expiredStr)
		options[i] = huh.NewOption(label, c)
	}
	return options
}

// promptForExtend selects expired containers to extend and by how many hours.
func promptForExtend(containers []*database.Container) ([]*database.Container, int, error) {
	var selected []*database.Container
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[*database.Container]().
				Title("⏰ Extend Expired Databases").
				Description("Select databases to keep (Space to toggle, Enter to continue)").
				Options(containerOptions(containers)...).
				Value(&selected),
		),
	).Run()
	if err != nil {
		return nil, 0, normalizeAbort(err)
	}

	if len(selected) == 0 {
		return nil, 0, nil
	}

	hoursStr := "24"
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Extend by how many hours?").
				Value(&hoursStr).
				Validate(validateHours),
		),
	).Run()
	if err != nil {
		return nil, 0, normalizeAbort(err)
	}

	hours, _ := strconv.Atoi(hoursStr)
	return selected, hours, nil
}

// promptForRemoval selects expired containers to remove.
func promptForRemoval(containers []*database.Container) ([]*database.Container, error) {
	var selected []*database.Container
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[*database.Container]().
				Title("🗑  Remove Expired Databases").
				Description("Select databases to remove (Space to toggle, Enter to confirm)").
				Options(containerOptions(containers)...).
				Value(&selected),
		),
	).Run()
	if err != nil {
		return nil, normalizeAbort(err)
	}
	return selected, nil
}

// validateHours checks that the input is a positive integer.
func validateHours(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("hours must be a whole number")
	}
	if n <= 0 {
		return fmt.Errorf("hours must be greater than 0")
	}
	return nil
}

func normalizeAbort(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return ui.ErrCancelled
	}
	return err
}

// formatExpiredDuration formats how long ago a container expired.
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

// extendContainer extends the TTL of a container, resetting from now if it has
// already expired.
func extendContainer(c *database.Container, hours int) error {
	if time.Now().After(c.ExpiresAt) {
		c.ExpiresAt = time.Now().Add(time.Duration(hours) * time.Hour)
	} else {
		c.ExpiresAt = c.ExpiresAt.Add(time.Duration(hours) * time.Hour)
	}

	if err := database.UpdateContainer(c); err != nil {
		return fmt.Errorf("failed to update container: %w", err)
	}
	config.Logger.Info("Container TTL extended", "name", c.DisplayName, "new_expiration", c.ExpiresAt)
	return nil
}

// cleanupContainer stops and removes a container, its volume, and its record.
func cleanupContainer(c *database.Container) error {
	config.Logger.Info("Cleaning up expired container", "name", c.DisplayName)

	if c.ContainerID != "" && docker.ContainerExists(c.ContainerID) {
		if err := docker.StopContainer(c.ContainerID); err != nil {
			config.Logger.Warn("Failed to stop container", "name", c.DisplayName, "error", err)
		}
		if err := docker.RemoveContainer(c.ContainerID); err != nil {
			config.Logger.Warn("Failed to remove container", "name", c.DisplayName, "error", err)
		}
	}

	if err := docker.RemoveVolume(c.VolumeType, c.VolumePath); err != nil {
		config.Logger.Warn("Failed to remove volume", "name", c.DisplayName, "error", err)
	}

	if err := database.DeleteContainer(c.ID); err != nil {
		return fmt.Errorf("failed to delete container from database: %w", err)
	}

	config.Logger.Info("Container cleanup complete", "name", c.DisplayName)
	return nil
}
