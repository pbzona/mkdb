package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/credentials"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/types"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var (
	userContainerName string
	userName          string
	userYes           bool
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage database users",
	Long:  `Add or remove database users. Not supported for Redis.`,
}

var userAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a database user",
	Long:  `Create a new user in the database with a generated password.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUserAdd,
}

var userRmCmd = &cobra.Command{
	Use:   "rm [name]",
	Short: "Remove a database user",
	Long:  `Delete a non-default user from the database.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUserRm,
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userAddCmd, userRmCmd)

	for _, c := range []*cobra.Command{userAddCmd, userRmCmd} {
		c.Flags().StringVar(&userContainerName, "name", "", "Container name (skips interactive selection)")
		c.Flags().StringVar(&userName, "username", "", "Username to add or remove")
	}
	userRmCmd.Flags().BoolVarP(&userYes, "yes", "y", false, "Skip the confirmation prompt")
}

// resolveUserContainer selects a running, user-manageable container.
func resolveUserContainer(args []string) (*database.Container, error) {
	container, err := resolveContainer(args, userContainerName, "Select container", func(c *database.Container) bool {
		return isRunning(c) && c.Type != types.DBTypeRedis
	})
	if errors.Is(err, errNoContainers) {
		ui.Warning("No running containers support user management")
		return nil, nil
	}
	if errors.Is(err, ui.ErrCancelled) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if container.Type == types.DBTypeRedis {
		return nil, fmt.Errorf("user management is not supported for Redis")
	}
	if !isRunning(container) {
		return nil, fmt.Errorf("container '%s' is not running", container.DisplayName)
	}
	return container, nil
}

func runUserAdd(cmd *cobra.Command, args []string) error {
	container, err := resolveUserContainer(args)
	if container == nil {
		return err
	}

	username := userName
	if username == "" {
		username, err = ui.PromptString("Enter username", "")
		if errors.Is(err, ui.ErrCancelled) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	if username == "" {
		return fmt.Errorf("username is required (pass --username)")
	}

	adminUser, adminPassword, err := adminCreds(container)
	if err != nil {
		return err
	}

	password, err := credentials.GeneratePassword(passwordLength)
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	if err := docker.CreateUser(container.ContainerID, container.Type, adminUser, adminPassword, username, password, container.DisplayName); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	encrypted, err := config.Encrypt(password)
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}

	if err := database.CreateUser(&database.User{
		ContainerID:  container.ID,
		Username:     username,
		PasswordHash: encrypted,
		IsDefault:    false,
		CreatedAt:    time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to store user: %w", err)
	}

	ui.Success(fmt.Sprintf("User '%s' created", username))
	fmt.Println(connectionString(container, username, password))
	return nil
}

func runUserRm(cmd *cobra.Command, args []string) error {
	container, err := resolveUserContainer(args)
	if container == nil {
		return err
	}

	user, err := selectRemovableUser(container, userName)
	if err != nil {
		return err
	}
	if user == nil {
		return nil
	}

	if !userYes {
		if !ui.IsInteractive() {
			return fmt.Errorf("refusing to remove user '%s' without confirmation; pass --yes", user.Username)
		}
		confirmed, err := ui.PromptConfirm(fmt.Sprintf("Delete user '%s'?", user.Username), false)
		if errors.Is(err, ui.ErrCancelled) {
			return nil
		}
		if err != nil {
			return err
		}
		if !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	adminUser, adminPassword, err := adminCreds(container)
	if err != nil {
		return err
	}

	if err := docker.DeleteUser(container.ContainerID, container.Type, adminUser, adminPassword, user.Username, container.DisplayName); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if err := database.DeleteUser(user.ID); err != nil {
		return fmt.Errorf("failed to remove user record: %w", err)
	}

	ui.Success(fmt.Sprintf("User '%s' deleted", user.Username))
	return nil
}

// selectRemovableUser finds the target non-default user by --username or via an
// interactive picker. Returns (nil, nil) when there is nothing to do.
func selectRemovableUser(container *database.Container, name string) (*database.User, error) {
	users, err := database.ListUsers(container.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	var candidates []*database.User
	for _, u := range users {
		if !u.IsDefault {
			candidates = append(candidates, u)
		}
	}
	if len(candidates) == 0 {
		ui.Warning("No removable users found (the default user cannot be deleted)")
		return nil, nil
	}

	if name != "" {
		for _, u := range candidates {
			if u.Username == name {
				return u, nil
			}
		}
		return nil, fmt.Errorf("user '%s' not found", name)
	}

	user, err := ui.SelectUser(candidates, "Select user to delete")
	if errors.Is(err, ui.ErrCancelled) {
		return nil, nil
	}
	return user, err
}
