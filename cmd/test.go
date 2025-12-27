package cmd

import (
	"fmt"

	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:     "test",
	Aliases: []string{"ping"},
	Short:   "Test database connectivity",
	Long:    `Test connectivity to a database container by running a simple query.`,
	RunE:    runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	// Get all containers
	containers, err := database.ListContainers()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		ui.Warning("No containers found")
		return nil
	}

	// Prompt user to select a container
	container, err := ui.SelectContainer(containers, "Select container to test")
	if err != nil {
		return fmt.Errorf("failed to select container: %w", err)
	}

	// Test connectivity based on database type
	ui.Info(fmt.Sprintf("Testing connectivity to %s (%s)...", container.DisplayName, container.Type))

	var testCommand []string
	switch container.Type {
	case "postgres":
		testCommand = []string{
			"psql",
			"-U", "dbuser",
			"-d", container.DisplayName,
			"-c", "SELECT 1 as status, current_user, current_database();",
		}
	case "mysql":
		testCommand = []string{
			"mysql",
			"-u", "dbuser",
			"-p$uper$ecret",
			container.DisplayName,
			"-e", "SELECT 1 as status, USER() as user, DATABASE() as db;",
		}
	case "redis":
		testCommand = []string{
			"redis-cli",
			"PING",
		}
	default:
		return fmt.Errorf("unsupported database type: %s", container.Type)
	}

	// Execute the test command
	output, err := docker.ExecCommand(container.Name, testCommand)
	if err != nil {
		ui.Error(fmt.Sprintf("Connection failed: %v", err))
		return fmt.Errorf("connectivity test failed: %w", err)
	}

	ui.Success("Connection successful!")
	fmt.Println()
	fmt.Println("Response:")
	fmt.Println(output)

	return nil
}
