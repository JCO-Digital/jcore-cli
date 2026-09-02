package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JCO-Digital/jcore/internal/constants"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// cloneCmd represents the clone command
var cloneCmd = &cobra.Command{
	Use:   "clone <repository> [name]",
	Short: "Clone an existing JCore project",
	Long: `Clones a JCore project from a Git repository and performs initial setup.
If only a name is provided, it uses the projectDefault setting to construct the Git URL.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]
		name := ""
		if len(args) > 1 {
			name = args[1]
		}

		// Handle shorthand project names
		if !strings.Contains(repo, ":") && !strings.Contains(repo, "/") {
			pattern := viper.GetString("projectDefault")
			if pattern == "" {
				pattern = "git@github.com:JCO-Digital/{name}.git"
			}
			if name == "" {
				name = repo
			}
			repo = strings.Replace(pattern, "{name}", repo, 1)
		}

		// If name still empty, derive from repo
		if name == "" {
			parts := strings.Split(repo, "/")
			lastPart := parts[len(parts)-1]
			name = strings.TrimSuffix(lastPart, ".git")
		}

		targetDir, _ := os.Getwd()
		targetDir = filepath.Join(targetDir, name)

		if _, err := os.Stat(targetDir); err == nil {
			fmt.Printf("Error: Target directory %s already exists.\n", targetDir)
			return
		}

		fmt.Printf("Cloning project %s from %s...\n", name, repo)
		gitClone := exec.Command("git", "clone", repo, targetDir)
		gitClone.Stdout = os.Stdout
		gitClone.Stderr = os.Stderr
		if err := gitClone.Run(); err != nil {
			fmt.Printf("Error during git clone: %v\n", err)
			return
		}

		// Initial setup after clone
		fmt.Println("Performing initial project setup...")

		// 1. Initialize submodules
		submoduleCmd := exec.Command("git", "submodule", "update", "--init", "--recursive")
		submoduleCmd.Dir = targetDir
		if err := submoduleCmd.Run(); err != nil {
			fmt.Printf("Warning: submodule init failed: %v\n", err)
		}

		// 2. Reload settings from the freshly cloned project's own config
		// files (jcore.toml, defaults.toml, ...): the config loaded at
		// startup only reflects wherever the command was run from, which
		// had no project yet.
		LoadConfigForProject(targetDir)

		// 3. Switch the jcore2 theme submodule to the configured branch,
		// if any.
		if branch := viper.GetString("branch"); branch != "" {
			themeDir := filepath.Join(targetDir, constants.JcoreThemeSubmodulePath)
			if _, err := os.Stat(themeDir); err == nil {
				switchCmd := exec.Command("git", "switch", branch)
				switchCmd.Dir = themeDir
				switchCmd.Stdout = os.Stdout
				switchCmd.Stderr = os.Stderr
				if err := switchCmd.Run(); err != nil {
					fmt.Printf("Warning: failed to switch jcore2 theme to branch %s: %v\n", branch, err)
				}
			}
		}

		// 4. Generate .env and finalize the project (nginx/php.ini
		// rendering), then install dependencies — always, regardless of
		// the `install` setting, matching the legacy CLI's
		// finalizeProject() call after a fresh clone.
		if err := project.GenerateEnvFile(targetDir); err != nil {
			fmt.Printf("Warning: Failed to generate initial .env: %v\n", err)
		}
		if err := project.FinalizeProject(targetDir); err != nil {
			fmt.Printf("Warning: Failed to finalize project: %v\n", err)
		}
		if err := project.InstallDependencies(targetDir, true); err != nil {
			fmt.Printf("Warning: Failed to install dependencies: %v\n", err)
		}

		fmt.Printf("\nProject %s cloned and prepared successfully.\n", name)
		fmt.Printf("Enter the project directory: cd %s\n", name)
	},
}

func init() {
	rootCmd.AddCommand(cloneCmd)
}
