package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/manifoldco/promptui"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/types"
)

var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true).Underline(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("12")).
			Padding(1, 2)
)

// Success prints a success message
func Success(message string) {
	fmt.Println(successStyle.Render("✓ " + message))
}

// Error prints an error message
func Error(message string) {
	fmt.Println(errorStyle.Render("✗ " + message))
}

// Warning prints a warning message
func Warning(message string) {
	fmt.Println(warningStyle.Render("⚠ " + message))
}

// Info prints an info message
func Info(message string) {
	fmt.Println(infoStyle.Render("ℹ " + message))
}

// Header prints a header
func Header(message string) {
	fmt.Println(headerStyle.Render(message))
}

// Box prints text in a box
func Box(content string) {
	fmt.Println(boxStyle.Render(content))
}

// SelectDBType prompts the user to select a database type
func SelectDBType() (string, error) {
	prompt := promptui.Select{
		Label: "Select database type",
		Items: types.ValidDBTypes(),
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "{{ . | green }}",
		},
		Keys: &promptui.SelectKeys{
			Prev:     promptui.Key{Code: promptui.KeyPrev, Display: "↑"},
			Next:     promptui.Key{Code: promptui.KeyNext, Display: "↓"},
			PageUp:   promptui.Key{Code: 'k'},
			PageDown: promptui.Key{Code: 'j'},
		},
	}

	_, result, err := prompt.Run()
	return result, err
}

// SelectContainer prompts the user to select a container
func SelectContainer(containers []*database.Container, label string) (*database.Container, error) {
	if len(containers) == 0 {
		return nil, fmt.Errorf("no containers found")
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .DisplayName | cyan }} ({{ .Type }})",
		Inactive: "  {{ .DisplayName }} ({{ .Type }})",
		Selected: "{{ .DisplayName | green }}",
	}

	prompt := promptui.Select{
		Label:     label,
		Items:     containers,
		Templates: templates,
		Keys: &promptui.SelectKeys{
			Prev:     promptui.Key{Code: promptui.KeyPrev, Display: "↑"},
			Next:     promptui.Key{Code: promptui.KeyNext, Display: "↓"},
			PageUp:   promptui.Key{Code: 'k'},
			PageDown: promptui.Key{Code: 'j'},
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	return containers[idx], nil
}

// SelectUser prompts the user to select a user
func SelectUser(users []*database.User, label string) (*database.User, error) {
	if len(users) == 0 {
		return nil, fmt.Errorf("no users found")
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .Username | cyan }}",
		Inactive: "  {{ .Username }}",
		Selected: "{{ .Username | green }}",
	}

	prompt := promptui.Select{
		Label:     label,
		Items:     users,
		Templates: templates,
		Keys: &promptui.SelectKeys{
			Prev:     promptui.Key{Code: promptui.KeyPrev, Display: "↑"},
			Next:     promptui.Key{Code: promptui.KeyNext, Display: "↓"},
			PageUp:   promptui.Key{Code: 'k'},
			PageDown: promptui.Key{Code: 'j'},
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	return users[idx], nil
}

// PromptString prompts the user for a string input
func PromptString(label string, defaultValue string) (string, error) {
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}

	return prompt.Run()
}

// PromptConfirm prompts the user for confirmation
func PromptConfirm(label string) (bool, error) {
	prompt := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}

	return strings.ToLower(result) == "y", nil
}

// SelectVolumeOption prompts the user to select a volume option
func SelectVolumeOption() (string, error) {
	prompt := promptui.Select{
		Label: "Do you want to create a volume for this database?",
		Items: []string{"none", "named", "custom path"},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "{{ . | green }}",
		},
		Keys: &promptui.SelectKeys{
			Prev:     promptui.Key{Code: promptui.KeyPrev, Display: "↑"},
			Next:     promptui.Key{Code: promptui.KeyNext, Display: "↓"},
			PageUp:   promptui.Key{Code: 'k'},
			PageDown: promptui.Key{Code: 'j'},
		},
	}

	_, result, err := prompt.Run()
	return result, err
}

// FormatDuration formats a duration in a human-readable way
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

// PrintContainerInfo prints detailed container information
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
