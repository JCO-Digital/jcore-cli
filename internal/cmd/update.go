package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [files...]",
	Short: "Update project files from templates",
	Long: `Updates the current project files from the embedded templates.
It respects manually modified files by checking their checksums.

If specific files are given, those are always overwritten. Otherwise, any
file whose checksum no longer matches (i.e. looks locally modified) is
presented as an interactive, unselected checklist - press enter to leave
them all as-is, or select the ones you want overwritten.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		var only, force []string

		if len(args) > 0 {
			only = args
			force = args
		} else {
			modified, err := project.DetectModifiedFiles(projectDir)
			if err != nil {
				fmt.Printf("Error checking for modified files: %v\n", err)
				return
			}

			if len(modified) > 0 {
				var selected []string
				prompt := &survey.MultiSelect{
					Message: "These files look locally modified (checksum mismatch). Select any you want overwritten:",
					Options: modified,
				}
				if err := survey.AskOne(prompt, &selected); err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}
				force = selected
			}
		}

		fmt.Println("Updating Project...")
		if err := project.UpdateProject(projectDir, only, force); err != nil {
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
