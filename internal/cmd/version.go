package cmd

import (
	"fmt"

	"github.com/JCO-Digital/jcore/internal/config"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of JCore CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("JCore CLI %s\n", config.AppVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
