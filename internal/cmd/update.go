package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JCO-Digital/jcore/internal/config"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/JCO-Digital/jcore/internal/update"
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

var (
	selfUpdateForce     bool
	selfUpdateCheckOnly bool
)

// updateSelfCmd represents the update self command
var updateSelfCmd = &cobra.Command{
	Use:   "self",
	Short: "Update the jcore CLI itself",
	Long: `Checks GitHub for a newer release of jcore, downloads the binary matching
this platform, verifies its signature, and replaces the running executable.`,
	Run: func(cmd *cobra.Command, args []string) {
		if selfUpdateCheckOnly {
			// Internal entry point used by the detached background checker.
			// Errors here are non-fatal; nothing is reading this process's
			// exit code.
			_ = update.RunCheckOnly(config.AppVersion)
			return
		}

		fmt.Printf("Current version: %s\n", config.AppVersion)

		exePath, err := os.Executable()
		if err != nil {
			fmt.Printf("Error determining executable path: %v\n", err)
			os.Exit(1)
		}
		exePath, err = filepath.EvalSymlinks(exePath)
		if err != nil {
			fmt.Printf("Error resolving executable path: %v\n", err)
			os.Exit(1)
		}

		latest, downloadURL, sigURL, available, err := update.CheckForUpdate(config.AppVersion)
		if err != nil {
			fmt.Printf("Error checking for updates: %v\n", err)
			os.Exit(1)
		}

		if !available {
			if selfUpdateForce && downloadURL != "" {
				fmt.Printf("Forcing reinstall of %s.\n", latest)
			} else if downloadURL == "" {
				fmt.Printf("No release asset found for this platform (%s).\n", update.AssetName())
				os.Exit(1)
			} else {
				fmt.Println("You are already running the latest version of jcore.")
				reportCompletions(installShellCompletions(exePath))
				return
			}
		} else {
			fmt.Printf("A new version is available: %s\n", latest)
		}

		confirmed := selfUpdateForce
		if !confirmed {
			prompt := &survey.Confirm{Message: "Download and install it?", Default: true}
			if err := survey.AskOne(prompt, &confirmed); err != nil || !confirmed {
				return
			}
		}

		fmt.Println("Downloading update...")
		if err := update.DownloadAndReplace(downloadURL, sigURL, exePath); err != nil {
			fmt.Printf("Update failed: %v\n", err)
			os.Exit(1)
		}

		// Refresh the check state now that we're current, so the notice
		// doesn't linger and the next background check isn't due immediately.
		_ = update.SaveState(update.State{Latest: latest, LastChecked: time.Now()})

		fmt.Printf("jcore updated to %s.\n", latest)

		// Regenerate completions from the newly-installed binary, not this
		// still-running (now stale) process, so they match the new version.
		reportCompletions(installShellCompletions(exePath))
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.AddCommand(updateSelfCmd)

	updateSelfCmd.Flags().BoolVarP(&selfUpdateForce, "force", "f", false, "Reinstall even if already on the latest version, skip confirmation")
	updateSelfCmd.Flags().BoolVar(&selfUpdateCheckOnly, "check-only", false, "Only check for an update and record the result (used internally)")
	_ = updateSelfCmd.Flags().MarkHidden("check-only")
}
