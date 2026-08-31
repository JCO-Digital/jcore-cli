package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

type DockerProject struct {
	Name        string `json:"Name"`
	Status      string `json:"Status"`
	ConfigFiles string `json:"ConfigFiles"`
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show information about running projects",
	Run: func(cmd *cobra.Command, args []string) {
		out, err := exec.Command("docker", "compose", "ls", "-a", "--format", "json").Output()
		if err != nil {
			fmt.Printf("Error checking Docker projects: %v\n", err)
			return
		}

		var projects []DockerProject
		if err := json.Unmarshal(out, &projects); err != nil {
			// Older docker compose might not return an array if there's only one project
			// or might have a slightly different format.
			var singleProject DockerProject
			if err := json.Unmarshal(out, &singleProject); err == nil {
				projects = []DockerProject{singleProject}
			} else {
				fmt.Printf("Error parsing Docker output: %v\n", err)
				return
			}
		}

		runningCount := 0
		for _, p := range projects {
			if strings.Contains(p.Status, "running") {
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
