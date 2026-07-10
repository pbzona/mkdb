package ui

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/types"
)

// ErrCancelled is returned by prompts when the user aborts (Ctrl+C / Esc).
// Callers should treat it as a clean, non-error cancellation.
var ErrCancelled = errors.New("cancelled")

var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("12")).
			Padding(1, 2)
)

// IsInteractive reports whether we can prompt the user: stdin must be a
// terminal (for input) and stderr must be a terminal (where prompts render).
func IsInteractive() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stderr.Fd())
}

// runForm runs a single huh field, rendering to stderr so that stdout is
// reserved for machine-readable output. Aborts are normalized to ErrCancelled.
func runForm(field huh.Field) error {
	err := huh.NewForm(huh.NewGroup(field)).WithOutput(os.Stderr).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCancelled
	}
	return err
}

// Status messages are written to stderr so they never pollute piped stdout.

// Success prints a success message.
func Success(message string) {
	fmt.Fprintln(os.Stderr, successStyle.Render("✓ "+message))
}

// Error prints an error message.
func Error(message string) {
	fmt.Fprintln(os.Stderr, errorStyle.Render("✗ "+message))
}

// Warning prints a warning message.
func Warning(message string) {
	fmt.Fprintln(os.Stderr, warningStyle.Render("⚠ "+message))
}

// Info prints an info message.
func Info(message string) {
	fmt.Fprintln(os.Stderr, infoStyle.Render("ℹ "+message))
}

// Box prints content in a box on stdout (used for primary command output).
func Box(content string) {
	fmt.Println(boxStyle.Render(content))
}

// SelectDBType prompts the user to select a database type.
func SelectDBType() (string, error) {
	if !IsInteractive() {
		return "", fmt.Errorf("no database type specified (pass a type argument, e.g. 'mkdb create postgres')")
	}

	dbTypes := types.ValidDBTypes()
	options := make([]huh.Option[string], len(dbTypes))
	for i, t := range dbTypes {
		options[i] = huh.NewOption(t, t)
	}

	var selected string
	if err := runForm(huh.NewSelect[string]().
		Title("Select database type").
		Options(options...).
		Value(&selected)); err != nil {
		return "", err
	}
	return selected, nil
}

// SelectContainer prompts the user to select a container.
func SelectContainer(containers []*database.Container, label string) (*database.Container, error) {
	if len(containers) == 0 {
		return nil, fmt.Errorf("no containers found")
	}
	if !IsInteractive() {
		return nil, fmt.Errorf("no container specified (pass a name argument or --name)")
	}

	options := make([]huh.Option[*database.Container], len(containers))
	for i, c := range containers {
		label := fmt.Sprintf("%s (%s)", c.DisplayName, c.Type)
		options[i] = huh.NewOption(label, c)
	}

	var selected *database.Container
	if err := runForm(huh.NewSelect[*database.Container]().
		Title(label).
		Options(options...).
		Value(&selected)); err != nil {
		return nil, err
	}
	return selected, nil
}

// SelectUser prompts the user to select a user.
func SelectUser(users []*database.User, label string) (*database.User, error) {
	if len(users) == 0 {
		return nil, fmt.Errorf("no users found")
	}
	if !IsInteractive() {
		return nil, fmt.Errorf("no user specified (pass --username)")
	}

	options := make([]huh.Option[*database.User], len(users))
	for i, u := range users {
		options[i] = huh.NewOption(u.Username, u)
	}

	var selected *database.User
	if err := runForm(huh.NewSelect[*database.User]().
		Title(label).
		Options(options...).
		Value(&selected)); err != nil {
		return nil, err
	}
	return selected, nil
}

// PromptString prompts the user for a string input.
func PromptString(label string, defaultValue string) (string, error) {
	if !IsInteractive() {
		return defaultValue, nil
	}

	value := defaultValue
	if err := runForm(huh.NewInput().
		Title(label).
		Value(&value)); err != nil {
		return "", err
	}
	return value, nil
}

// PromptConfirm prompts the user for confirmation. When not interactive it
// returns defaultValue without prompting.
func PromptConfirm(label string, defaultValue bool) (bool, error) {
	if !IsInteractive() {
		return defaultValue, nil
	}

	value := defaultValue
	if err := runForm(huh.NewConfirm().
		Title(label).
		Affirmative("Yes").
		Negative("No").
		Value(&value)); err != nil {
		return false, err
	}
	return value, nil
}

// SelectVolumeOption prompts the user to select a volume option.
func SelectVolumeOption() (string, error) {
	if !IsInteractive() {
		return types.VolumeTypeNone, nil
	}

	var selected string
	if err := runForm(huh.NewSelect[string]().
		Title("Create a volume for this database?").
		Options(
			huh.NewOption("No volume (data is lost on removal)", types.VolumeTypeNone),
			huh.NewOption("Named volume (managed by mkdb)", types.VolumeTypeNamed),
			huh.NewOption("Custom path (bind mount)", types.VolumeTypeCustom),
		).
		Value(&selected)); err != nil {
		return "", err
	}
	return selected, nil
}

// FormatDuration formats a duration in a human-readable way.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}

	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// PrintContainerInfo prints detailed container information.
func PrintContainerInfo(c *database.Container) {
	timeRemaining := time.Until(c.ExpiresAt)

	info := fmt.Sprintf(`Name:        %s
Type:        %s
Version:     %s
Status:      %s
Port:        %s
Created:     %s
Expires:     %s (%s remaining)
Volume:      %s`,
		c.DisplayName,
		c.Type,
		c.Version,
		c.Status,
		c.Port,
		c.CreatedAt.Format("2006-01-02 15:04:05"),
		c.ExpiresAt.Format("2006-01-02 15:04:05"),
		FormatDuration(timeRemaining),
		formatVolumeInfo(c),
	)

	Box(info)
}

func formatVolumeInfo(c *database.Container) string {
	if c.VolumeType == "" {
		return "none"
	}
	return fmt.Sprintf("%s (%s)", c.VolumePath, c.VolumeType)
}
