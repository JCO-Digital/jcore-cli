package cmd

import (
	"fmt"

	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean [all|docker]",
	Short: "Clean containers and volumes for the current project",
	Long: `Cleans up Docker resources.
With no argument, cleans containers, volumes, and .jcore workfiles for the current project.
"all" cleans every non-running JCore project and prunes Docker globally.
"docker" prunes unused Docker containers, images, volumes, and networks.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := ""
		if len(args) > 0 {
			target = args[0]
		}

		switch target {
		case "all":
			if err := project.CleanAll(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case "docker":
			if err := project.CleanDocker(false); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		default:
			projectDir, err := project.FindProjectRoot()
			if err != nil || projectDir == "" {
				fmt.Println("Error: not in a JCore project (no jcore.toml found)")
				return
			}

			// Docker labels the volumes with the compose project name, which
			// comes from the directory, not the projectName setting.
			name := project.ComposeProjectName(projectDir)
			if err := project.CleanProject(project.DockerProject{Name: name, Path: projectDir}); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
