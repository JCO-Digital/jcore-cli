package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show information about running projects",
	Run: func(cmd *cobra.Command, args []string) {
		running, err := runningProjects()
		if err != nil {
			fmt.Printf("Error checking Docker projects: %v\n", err)
			return
		}

		if len(running) == 0 {
			fmt.Println("No running projects.")
			return
		}
		for _, p := range running {
			fmt.Printf("Project %s is running.\n", p.Name)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
