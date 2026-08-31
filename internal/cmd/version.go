package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "0.0.1"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of JCore CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("JCore CLI v%s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
