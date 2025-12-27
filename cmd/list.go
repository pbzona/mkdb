package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/types"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/pbzona/mkdb/internal/volumes"
	"github.com/spf13/cobra"
)

var (
	filterType   string
	filterStatus string
	showAll      bool
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all database containers",
	Long:    `List all database containers with optional filtering by type and status.`,
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&filterType, "type", "", "Filter by database type (postgres, mysql, redis)")
	listCmd.Flags().StringVar(&filterStatus, "status", "", "Filter by status (running, stopped, expired, removed)")
	listCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all databases including removed ones")
}

func runList(cmd *cobra.Command, args []string) error {
	// Get all containers
	containers, err := database.ListContainers()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Check for orphaned volumes and add them as "removed" containers
	if showAll || filterStatus == "removed" {
		orphaned, err := volumes.ScanOrphaned()
		if err != nil {
			return fmt.Errorf("failed to scan volumes: %w", err)
		}

		// Convert orphaned volumes to container objects with "removed" status
		for _, vol := range orphaned {
			removedContainer := &database.Container{
				DisplayName: vol.Name,
				Status:      "removed",
				VolumeType:  "named",
				VolumePath:  vol.Name,
				CreatedAt:   vol.ModTime,                      // Use volume modification time
				ExpiresAt:   time.Now().Add(1000 * time.Hour), // Set far future so it doesn't show as expired
			}

			// If we have original container info, use it
			if vol.Container != nil {
				removedContainer.Type = vol.Container.Type
				removedContainer.Version = vol.Container.Version
				removedContainer.CreatedAt = vol.Container.CreatedAt
				removedContainer.ExpiresAt = vol.Container.ExpiresAt
				removedContainer.Port = vol.Container.Port
			}

			containers = append(containers, removedContainer)
		}
	}

	if len(containers) == 0 {
		ui.Warning("No containers found")
		return nil
	}

	// Apply filters
	filtered := filterContainers(containers, filterType, filterStatus)

	if len(filtered) == 0 {
		ui.Warning(fmt.Sprintf("No containers found matching filters (type=%s, status=%s)",
			valueOrAny(filterType), valueOrAny(filterStatus)))
		return nil
	}

	// Display results
	displayContainerList(filtered)

	return nil
}

func filterContainers(containers []*database.Container, typeFilter, statusFilter string) []*database.Container {
	var filtered []*database.Container

	for _, c := range containers {
		// Filter by type
		if typeFilter != "" {
			normalizedType := normalizeType(c.Type)
			normalizedFilter := normalizeType(typeFilter)
			if normalizedType != normalizedFilter {
				continue
			}
		}

		// Filter by status
		if statusFilter != "" {
			normalizedStatus := normalizeStatus(c, statusFilter)
			if !normalizedStatus {
				continue
			}
		}

		filtered = append(filtered, c)
	}

	return filtered
}

func normalizeType(dbType string) string {
	normalized, err := types.NormalizeDBType(dbType)
	if err != nil {
		return dbType // Return as-is if invalid
	}
	return normalized
}

func normalizeStatus(c *database.Container, statusFilter string) bool {
	statusFilter = strings.ToLower(strings.TrimSpace(statusFilter))

	// If status is explicitly "removed", don't override it
	if c.Status == "removed" {
		return statusFilter == "" || statusFilter == "removed"
	}

	// Check if expired
	isExpired := time.Now().After(c.ExpiresAt)
	actualStatus := c.Status
	if isExpired && c.Status != "stopped" {
		actualStatus = "expired"
	}

	switch statusFilter {
	case "up", "running":
		return actualStatus == "running"
	case "down", "stopped":
		return actualStatus == "stopped"
	case "expired":
		return actualStatus == "expired"
	case "removed":
		return c.Status == "removed"
	default:
		return true
	}
}

func displayContainerList(containers []*database.Container) {
	// Define styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	statusRunningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true) // Green
	statusStoppedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true) // Yellow
	statusExpiredStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)  // Red
	statusRemovedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)  // Gray

	// Calculate column widths
	nameWidth := max(len("NAME"), maxLen(containers, func(c *database.Container) string { return c.DisplayName }))
	typeWidth := max(len("TYPE"), maxLen(containers, func(c *database.Container) string { return c.Type }))
	portWidth := max(len("PORT"), maxLen(containers, func(c *database.Container) string { return c.Port }))

	// Print header
	fmt.Println()
	// Build header with proper padding then style it
	header := fmt.Sprintf("%-*s  %-*s  %-10s  %-*s  %s",
		nameWidth, "NAME",
		typeWidth, "TYPE",
		"STATUS",
		portWidth, "PORT",
		"TTL REMAINING")
	fmt.Println(headerStyle.Render(header))

	// Print separator
	totalWidth := nameWidth + typeWidth + 10 + portWidth + 15 + 8 // +8 for spacing
	fmt.Println(strings.Repeat("─", totalWidth))

	// Print rows
	for _, c := range containers {
		// Determine actual status - don't override "removed" status
		displayStatus := c.Status
		if c.Status != "removed" {
			isExpired := time.Now().After(c.ExpiresAt)
			if isExpired && c.Status != "stopped" {
				displayStatus = "expired"
			}
		}

		// Format TTL
		ttlRemaining := formatTTL(c)

		// Apply status style
		var styledStatus string
		switch displayStatus {
		case "running":
			styledStatus = statusRunningStyle.Render("● running")
		case "stopped":
			styledStatus = statusStoppedStyle.Render("● stopped")
		case "expired":
			styledStatus = statusExpiredStyle.Render("● expired")
		case "removed":
			styledStatus = statusRemovedStyle.Render("○ removed")
		default:
			styledStatus = displayStatus
		}

		// Print row - use plain printf with spacing
		fmt.Printf("%-*s  %-*s  %s  %-*s  %s\n",
			nameWidth, c.DisplayName,
			typeWidth, c.Type,
			padStatus(styledStatus, 10),
			portWidth, c.Port,
			ttlRemaining)
	}

	fmt.Println()
	fmt.Printf("Total: %d container(s)\n", len(containers))
	fmt.Println()
}

// padStatus adds padding to a styled status string while accounting for ANSI codes
func padStatus(styledStatus string, width int) string {
	visibleLen := len("● running") // All statuses are this length
	padding := width - visibleLen
	if padding < 0 {
		padding = 0
	}
	return styledStatus + strings.Repeat(" ", padding)
}

// Helper function to find max length
func maxLen(containers []*database.Container, fn func(*database.Container) string) int {
	maxL := 0
	for _, c := range containers {
		if l := len(fn(c)); l > maxL {
			maxL = l
		}
	}
	return maxL
}

// Helper function for max of two ints
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatTTL(c *database.Container) string {
	timeRemaining := time.Until(c.ExpiresAt)

	if timeRemaining < 0 {
		return "expired"
	}

	hours := int(timeRemaining.Hours())
	minutes := int(timeRemaining.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		if hours > 0 {
			return fmt.Sprintf("%dd %dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}

func valueOrAny(s string) string {
	if s == "" {
		return "any"
	}
	return s
}
