package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var configName string

var configCmd = &cobra.Command{
	Use:   "config [name]",
	Short: "Edit a database's configuration file",
	Long:  `Open a database container's configuration file in $EDITOR (default: vi).`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().StringVar(&configName, "name", "", "Container name (skips interactive selection)")
}

func runConfig(cmd *cobra.Command, args []string) error {
	container, err := resolveContainer(args, configName, "Select container to configure", nil)
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

	configDir := filepath.Join(config.DataDir, "configs", container.DisplayName)
	configFile := filepath.Join(configDir, docker.GetConfigFileName(container.Type))

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", configFile)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	ui.Info(fmt.Sprintf("Opening %s in %s...", configFile, editor))

	editorCmd := exec.Command(editor, configFile)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	ui.Info(fmt.Sprintf("Restart the container to apply changes: mkdb restart %s", container.DisplayName))
	return nil
}
