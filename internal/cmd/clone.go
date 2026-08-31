package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		_ = submoduleCmd.Run()

		// 2. Generate .env
		// We need to reload viper to pick up the newly cloned project's config
		// But for now we can just use the project.GenerateEnvFile logic
		// if we ensure it reads from the right place.
		if err := project.GenerateEnvFile(targetDir); err != nil {
			fmt.Printf("Warning: Failed to generate initial .env: %v\n", err)
		}

		fmt.Printf("\nProject %s cloned and prepared successfully.\n", name)
		fmt.Printf("Enter the project directory: cd %s\n", name)
	},
}

func init() {
	rootCmd.AddCommand(cloneCmd)
}
