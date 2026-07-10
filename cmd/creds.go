package cmd

import (
	"errors"
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/credentials"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var credsName string

var credsCmd = &cobra.Command{
	Use:   "creds",
	Short: "Manage database credentials",
	Long:  `Show, copy, or rotate credentials for the default database user.`,
}

var credsShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Print the connection string",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCredsShow,
}

var credsCopyCmd = &cobra.Command{
	Use:   "copy [name]",
	Short: "Copy the connection string to the clipboard",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCredsCopy,
}

var credsRotateCmd = &cobra.Command{
	Use:   "rotate [name]",
	Short: "Rotate the default user's password",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCredsRotate,
}

func init() {
	rootCmd.AddCommand(credsCmd)
	credsCmd.AddCommand(credsShowCmd, credsCopyCmd, credsRotateCmd)

	for _, c := range []*cobra.Command{credsShowCmd, credsCopyCmd, credsRotateCmd} {
		c.Flags().StringVar(&credsName, "name", "", "Container name (skips interactive selection)")
	}
}

func runCredsShow(cmd *cobra.Command, args []string) error {
	container, err := resolveCredsContainer(args)
	if container == nil {
		return err
	}

	connStr, err := defaultConnectionString(container)
	if err != nil {
		return err
	}

	fmt.Println(connStr)
	return nil
}

func runCredsCopy(cmd *cobra.Command, args []string) error {
	container, err := resolveCredsContainer(args)
	if container == nil {
		return err
	}

	connStr, err := defaultConnectionString(container)
	if err != nil {
		return err
	}

	if err := clipboard.WriteAll(connStr); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	ui.Success("Connection string copied to clipboard")
	return nil
}

func runCredsRotate(cmd *cobra.Command, args []string) error {
	container, err := resolveContainer(args, credsName, "Select container", isRunning)
	if errors.Is(err, errNoContainers) {
		ui.Warning("No running containers found")
		return nil
	}
	if errors.Is(err, ui.ErrCancelled) {
		return nil
	}
	if err != nil {
		return err
	}
	if !isRunning(container) {
		return fmt.Errorf("container '%s' is not running", container.DisplayName)
	}

	user, err := database.GetDefaultUser(container.ID)
	if err != nil {
		return fmt.Errorf("failed to get default user: %w", err)
	}
	if user.Username == "" || user.PasswordHash == "" {
		return fmt.Errorf("cannot rotate credentials for an unauthenticated database")
	}

	adminUser, adminPassword, err := adminCreds(container)
	if err != nil {
		return err
	}

	newPassword, err := credentials.GeneratePassword(passwordLength)
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	if err := docker.RotatePassword(container.ContainerID, container.Type, adminUser, adminPassword, user.Username, newPassword, container.DisplayName); err != nil {
		return fmt.Errorf("failed to rotate password: %w", err)
	}

	encrypted, err := config.Encrypt(newPassword)
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}
	user.PasswordHash = encrypted
	if err := database.UpdateUser(user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	ui.Success("Password rotated")
	fmt.Println(connectionString(container, user.Username, newPassword))
	return nil
}

// resolveCredsContainer resolves the target for show/copy. It returns a nil
// container (and any error) when the caller should stop.
func resolveCredsContainer(args []string) (*database.Container, error) {
	container, err := resolveContainer(args, credsName, "Select container", nil)
	if errors.Is(err, errNoContainers) {
		ui.Warning("No containers found")
		return nil, nil
	}
	if errors.Is(err, ui.ErrCancelled) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return container, nil
}
