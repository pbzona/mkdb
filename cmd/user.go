package cmd

import (
	"fmt"
	"time"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/credentials"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var (
	userContainerName string
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage database users",
	Long:  `Create or delete database users.`,
}

var userCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new database user",
	Long:  `Create a new user in the database with a generated password.`,
	RunE:  runUserCreate,
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an existing database user",
	Long:  `Delete a user from the database.`,
	RunE:  runUserDelete,
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userCreateCmd)
	userCmd.AddCommand(userDeleteCmd)

	// Add --name flag to user subcommands
	userCreateCmd.Flags().StringVar(&userContainerName, "name", "", "Container name (skips interactive selection)")
	userDeleteCmd.Flags().StringVar(&userContainerName, "name", "", "Container name (skips interactive selection)")
}

func runUserCreate(cmd *cobra.Command, args []string) error {
	var container *database.Container
	var err error

	// If name is provided, look it up directly
	if userContainerName != "" {
		container, err = database.GetContainerByDisplayName(userContainerName)
		if err != nil {
			return fmt.Errorf("container '%s' not found", userContainerName)
		}
		if container.Status != "running" {
			return fmt.Errorf("container '%s' is not running", userContainerName)
		}
	} else {
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
		container, err = ui.SelectContainer(running, "Select container")
		if err != nil {
			return fmt.Errorf("failed to select container: %w", err)
		}
	}

	// Prompt for username
	username, err := ui.PromptString("Enter username", "")
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}

	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	ui.Info("Generating password...")

	// Generate password
	password, err := credentials.GeneratePassword(32)
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	// Create user in database container
	if err := docker.CreateUser(container.ContainerID, container.Type, username, password, container.DisplayName); err != nil {
		return fmt.Errorf("failed to create user in database: %w", err)
	}

	// Encrypt and store password
	encryptedPassword, err := config.Encrypt(password)
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}

	user := &database.User{
		ContainerID:  container.ID,
		Username:     username,
		PasswordHash: encryptedPassword,
		IsDefault:    false,
		CreatedAt:    time.Now(),
	}

	if err := database.CreateUser(user); err != nil {
		return fmt.Errorf("failed to store user: %w", err)
	}

	ui.Success(fmt.Sprintf("User '%s' created successfully!", username))

	// Display connection string
	connStr := credentials.FormatConnectionString(
		container.Type,
		username,
		password,
		"localhost",
		container.Port,
		container.DisplayName,
	)

	ui.Box(credentials.FormatEnvVar(connStr))
	return nil
}

func runUserDelete(cmd *cobra.Command, args []string) error {
	var container *database.Container
	var err error

	// If name is provided, look it up directly
	if userContainerName != "" {
		container, err = database.GetContainerByDisplayName(userContainerName)
		if err != nil {
			return fmt.Errorf("container '%s' not found", userContainerName)
		}
		if container.Status != "running" {
			return fmt.Errorf("container '%s' is not running", userContainerName)
		}
	} else {
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
		container, err = ui.SelectContainer(running, "Select container")
		if err != nil {
			return fmt.Errorf("failed to select container: %w", err)
		}
	}

	// Get users for this container
	users, err := database.ListUsers(container.ID)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	// Filter out default user
	var nonDefaultUsers []*database.User
	for _, u := range users {
		if !u.IsDefault {
			nonDefaultUsers = append(nonDefaultUsers, u)
		}
	}

	if len(nonDefaultUsers) == 0 {
		ui.Warning("No non-default users found")
		return nil
	}

	// Select user
	user, err := ui.SelectUser(nonDefaultUsers, "Select user to delete")
	if err != nil {
		return fmt.Errorf("failed to select user: %w", err)
	}

	// Confirm deletion
	confirmed, err := ui.PromptConfirm(fmt.Sprintf("Are you sure you want to delete user '%s'?", user.Username))
	if err != nil {
		return fmt.Errorf("failed to get confirmation: %w", err)
	}

	if !confirmed {
		ui.Info("Deletion cancelled")
		return nil
	}

	// Delete user from database container
	if err := docker.DeleteUser(container.ContainerID, container.Type, user.Username, container.DisplayName); err != nil {
		return fmt.Errorf("failed to delete user from database: %w", err)
	}

	// Delete from our database
	if err := database.DeleteUser(user.ID); err != nil {
		return fmt.Errorf("failed to delete user from database: %w", err)
	}

	ui.Success(fmt.Sprintf("User '%s' deleted successfully!", user.Username))
	return nil
}
