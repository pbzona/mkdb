package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the current version of mkdb, overridable at build time via
// -ldflags "-X github.com/pbzona/mkdb/cmd.Version=...".
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the mkdb version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mkdb %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Wire up `mkdb --version` / `mkdb -v`. Cobra adds the -v shorthand
	// automatically because it is otherwise unused, and it short-circuits
	// before PersistentPreRunE, so no config/Docker init is required.
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("mkdb {{.Version}}\n")
}
