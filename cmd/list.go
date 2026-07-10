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
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List database containers",
	Long:    `List database containers, optionally filtered by type and status.`,
	Args:    cobra.NoArgs,
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&filterType, "type", "", "Filter by database type (postgres, mysql, redis)")
	listCmd.Flags().StringVar(&filterStatus, "status", "", "Filter by status (running, stopped, expired, removed)")
	listCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Include removed databases with orphaned volumes")
}

// listEntry is a unified row for the table, covering both tracked containers
// and orphaned volumes shown as "removed".
type listEntry struct {
	name    string
	dbType  string
	status  string
	port    string
	ttl     string
	size    string
	removed bool
}

func runList(cmd *cobra.Command, args []string) error {
	var normalizedStatus string
	if filterStatus != "" {
		s, err := types.NormalizeStatus(filterStatus)
		if err != nil {
			return err
		}
		normalizedStatus = s
	}

	var normalizedType string
	if filterType != "" {
		t, err := types.NormalizeDBType(filterType)
		if err != nil {
			return err
		}
		normalizedType = t
	}

	containers, err := database.ListContainers()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	var entries []listEntry
	for _, c := range containers {
		entries = append(entries, listEntry{
			name:   c.DisplayName,
			dbType: c.Type,
			status: effectiveStatus(c),
			port:   c.Port,
			ttl:    formatTTL(c),
		})
	}

	if showAll || normalizedStatus == types.StatusRemoved {
		orphaned, err := volumes.ScanOrphaned()
		if err != nil {
			return fmt.Errorf("failed to scan volumes: %w", err)
		}
		for _, vol := range orphaned {
			entries = append(entries, listEntry{
				name:    vol.Name,
				dbType:  "-",
				status:  types.StatusRemoved,
				port:    "-",
				ttl:     "-",
				size:    volumes.FormatSize(vol.Size),
				removed: true,
			})
		}
	}

	entries = filterEntries(entries, normalizedType, normalizedStatus)
	if len(entries) == 0 {
		ui.Warning("No matching containers found")
		return nil
	}

	displayEntries(entries)
	return nil
}

// effectiveStatus reflects TTL expiry on top of the stored status.
func effectiveStatus(c *database.Container) string {
	if c.Status == types.StatusRunning && time.Now().After(c.ExpiresAt) {
		return types.StatusExpired
	}
	return c.Status
}

func filterEntries(entries []listEntry, typeFilter, statusFilter string) []listEntry {
	var out []listEntry
	for _, e := range entries {
		if typeFilter != "" && e.dbType != typeFilter {
			continue
		}
		if statusFilter != "" && e.status != statusFilter {
			continue
		}
		out = append(out, e)
	}
	return out
}

func displayEntries(entries []listEntry) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	statusStyles := map[string]lipgloss.Style{
		types.StatusRunning: lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
		types.StatusStopped: lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true),
		types.StatusExpired: lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		types.StatusRemoved: lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true),
	}
	statusGlyph := map[string]string{
		types.StatusRunning: "●",
		types.StatusStopped: "●",
		types.StatusExpired: "●",
		types.StatusRemoved: "○",
	}

	nameWidth := columnWidth("NAME", entries, func(e listEntry) string { return e.name })
	typeWidth := columnWidth("TYPE", entries, func(e listEntry) string { return e.dbType })
	portWidth := columnWidth("PORT", entries, func(e listEntry) string { return e.port })
	const statusWidth = 10

	fmt.Println()
	fmt.Println(headerStyle.Render(fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %s",
		nameWidth, "NAME", typeWidth, "TYPE", statusWidth, "STATUS", portWidth, "PORT", "TTL / SIZE")))
	fmt.Println(strings.Repeat("─", nameWidth+typeWidth+statusWidth+portWidth+len("TTL / SIZE")+8))

	for _, e := range entries {
		label := fmt.Sprintf("%s %s", statusGlyph[e.status], e.status)
		styled := label
		if style, ok := statusStyles[e.status]; ok {
			styled = style.Render(label)
		}
		// Pad based on the visible (unstyled) label length.
		styled += strings.Repeat(" ", max(0, statusWidth-len(label)))

		trailing := e.ttl
		if e.removed {
			trailing = e.size
		}

		fmt.Printf("%-*s  %-*s  %s  %-*s  %s\n",
			nameWidth, e.name, typeWidth, e.dbType, styled, portWidth, e.port, trailing)
	}

	fmt.Println()
	fmt.Printf("Total: %d container(s)\n", len(entries))
	fmt.Println()
}

func columnWidth(header string, entries []listEntry, fn func(listEntry) string) int {
	width := len(header)
	for _, e := range entries {
		if l := len(fn(e)); l > width {
			width = l
		}
	}
	return width
}

func formatTTL(c *database.Container) string {
	remaining := time.Until(c.ExpiresAt)
	if remaining < 0 {
		return "expired"
	}

	hours := int(remaining.Hours())
	minutes := int(remaining.Minutes()) % 60

	if hours >= 24 {
		days := hours / 24
		hours %= 24
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
