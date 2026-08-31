package cmd

import (
	"fmt"

	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [files...]",
	Short: "Update project files from templates",
	Long: `Updates the current project files from the embedded templates.
It respects manually modified files by checking their checksums.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		fmt.Println("Updating Project...")
		if err := project.UpdateProject(projectDir, args); err != nil {
			fmt.Printf("Error during update: %v\n", err)
			return
		}
		if err := project.FinalizeProject(projectDir); err != nil {
			fmt.Printf("Error finalizing project: %v\n", err)
			return
		}
		fmt.Println("Update Finished.")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
