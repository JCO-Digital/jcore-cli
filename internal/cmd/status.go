package cmd

import (
	"fmt"

	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show information about running projects",
	Run: func(cmd *cobra.Command, args []string) {
		projects, err := project.ListDockerProjects()
		if err != nil {
			fmt.Printf("Error checking Docker projects: %v\n", err)
			return
		}

		runningCount := 0
		for _, p := range projects {
			if p.Running {
				fmt.Printf("Project %s is running.\n", p.Name)
				runningCount++
			}
		}

		if runningCount == 0 {
			fmt.Println("No running projects.")
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
