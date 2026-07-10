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
}
