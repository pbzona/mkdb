package cmd

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/credentials"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var credsCmd = &cobra.Command{
	Use:   "creds",
	Short: "Manage database credentials",
	Long:  `Get or rotate credentials for database users.`,
}

var credsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get connection string for the default user",
	Long:  `Display the connection string for the default database user.`,
	RunE:  runCredsGet,
}

var credsCopyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy connection string to clipboard",
	Long:  `Copy the connection string for the default database user to the clipboard.`,
	RunE:  runCredsCopy,
}

var credsRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate credentials for the default user",
	Long:  `Generate a new password for the default user and update it in the database.`,
	RunE:  runCredsRotate,
}

func init() {
	rootCmd.AddCommand(credsCmd)
	credsCmd.AddCommand(credsGetCmd)
	credsCmd.AddCommand(credsCopyCmd)
	credsCmd.AddCommand(credsRotateCmd)
}

func runCredsGet(cmd *cobra.Command, args []string) error {
	envVar, err := getConnectionString()
	if err != nil {
		return err
	}

	// Print the connection string
	fmt.Println(envVar)
	return nil
}

func runCredsCopy(cmd *cobra.Command, args []string) error {
	envVar, err := getConnectionString()
	if err != nil {
		return err
	}

	// Copy to clipboard
	if err := clipboard.WriteAll(envVar); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	ui.Success("Connection string copied to clipboard!")
	return nil
}

func getConnectionString() (string, error) {
	// Get all containers
	containers, err := database.ListContainers()
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		ui.Warning("No containers found")
		return "", fmt.Errorf("no containers found")
	}

	// Select container
	container, err := ui.SelectContainer(containers, "Select container")
	if err != nil {
		return "", fmt.Errorf("failed to select container: %w", err)
	}

	// Get default user
	user, err := database.GetDefaultUser(container.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get default user: %w", err)
	}

	// Decrypt password
	password, err := config.Decrypt(user.PasswordHash)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt password: %w", err)
	}

	// Format connection string
	connStr := credentials.FormatConnectionString(
		container.Type,
		user.Username,
		password,
		"localhost",
		container.Port,
		container.DisplayName,
	)

	return credentials.FormatEnvVar(connStr), nil
}

func runCredsRotate(cmd *cobra.Command, args []string) error {
	// Get all containers
	containers, err := database.ListContainers()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Filter running containers
	var running []*database.Container
	for _, c := range containers {
		if c.Status == "running" {
			running = append(running, c)
		}
	}

	if len(running) == 0 {
		ui.Warning("No running containers found")
		return nil
	}

	// Select container
	container, err := ui.SelectContainer(running, "Select container")
	if err != nil {
		return fmt.Errorf("failed to select container: %w", err)
	}

	// Get default user
	user, err := database.GetDefaultUser(container.ID)
	if err != nil {
		return fmt.Errorf("failed to get default user: %w", err)
	}

	ui.Info("Generating new password...")

	// Generate new password
	newPassword, err := credentials.GeneratePassword(32)
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	// Update password in database container
	if err := docker.RotatePassword(container.ContainerID, container.Type, user.Username, newPassword, container.DisplayName); err != nil {
		return fmt.Errorf("failed to rotate password in database: %w", err)
	}

	// Encrypt and store new password
	encryptedPassword, err := config.Encrypt(newPassword)
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}

	user.PasswordHash = encryptedPassword
	if err := database.UpdateUser(user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	ui.Success("Password rotated successfully!")

	// Display new connection string
	connStr := credentials.FormatConnectionString(
		container.Type,
		user.Username,
		newPassword,
		"localhost",
		container.Port,
		container.DisplayName,
	)

	envVar := credentials.FormatEnvVar(connStr)

	// Print the connection string
	fmt.Println(envVar)
	return nil
}
