package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean [all|docker]",
	Short: "Clean containers and volumes for the current project",
	Long: `Cleans up Docker resources.
With no argument, cleans containers, volumes, and .jcore workfiles for the current project.
"all" does the same for every JCore project that isn't running, then removes
dangling images. It asks first, listing the projects.
"docker" prunes Docker machine-wide, not just JCore: stopped containers,
dangling images, anonymous volumes and unused networks. It asks first.
Use --yes to skip the questions.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := ""
		if len(args) > 0 {
			target = args[0]
		}

		switch target {
		case "all":
			projects, err := project.StoppedProjects()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			if len(projects) == 0 {
				fmt.Println("No stopped JCore projects to clean.")
				return
			}
			fmt.Println("This deletes the containers, volumes (including the database) and .jcore files of:")
			for _, p := range projects {
				fmt.Printf("  %s (%s)\n", p.Name, p.Path)
			}
			if !confirmClean(cmd, fmt.Sprintf("Clean these %d projects?", len(projects))) {
				return
			}
			if err := project.CleanProjects(projects); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case "docker":
			fmt.Println("This prunes Docker on this whole machine, not only JCore: stopped containers,")
			fmt.Println("dangling images, anonymous volumes and unused networks.")
			if !confirmClean(cmd, "Prune Docker?") {
				return
			}
			if err := project.CleanDocker(); err != nil {
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

// confirmClean asks message, defaulting to no, unless --yes was given. A
// failed prompt (e.g. no terminal) counts as no.
func confirmClean(cmd *cobra.Command, message string) bool {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return true
	}
	confirmed := false
	if err := survey.AskOne(&survey.Confirm{Message: message, Default: false}, &confirmed); err != nil || !confirmed {
		fmt.Println("Nothing cleaned.")
		return false
	}
	return true
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolP("yes", "y", false, "Don't ask for confirmation")
}
