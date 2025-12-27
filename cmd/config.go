package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Edit database configuration file",
	Long:  `Open the database configuration file in your default editor ($EDITOR).`,
	RunE:  runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
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
	container, err := ui.SelectContainer(containers, "Select container to configure")
	if err != nil {
		return fmt.Errorf("failed to select container: %w", err)
	}

	// Get config file path
	configDir := filepath.Join(config.DataDir, "configs", container.DisplayName)
	configFile := filepath.Join(configDir, docker.GetConfigFileName(container.Type))

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", configFile)
	}

	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // Default to vi
	}

	ui.Info(fmt.Sprintf("Opening %s in %s...", configFile, editor))

	// Open editor
	editorCmd := exec.Command(editor, configFile)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	// Print restart command
	fmt.Println()
	ui.Info("To apply configuration changes, restart the container:")
	fmt.Printf("  mkdb restart\n")
	fmt.Println()

	return nil
}
