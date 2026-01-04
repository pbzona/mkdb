package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version is the current version of mkdb
	// This can be overridden at build time with -ldflags
	Version = "dev"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of mkdb",
	Long:  `Display the current version of mkdb.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mkdb %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
