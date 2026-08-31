package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate a legacy JCore project to the current format",
	Long: `Converts a legacy config.sh-based project to the current jcore.toml format,
then updates project files (docker-compose.yml, etc.) from the current templates.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, isLegacy, err := project.FindLegacyProjectRoot()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if projectDir == "" || !isLegacy {
			fmt.Println("Error: no legacy project (config.sh) found to migrate.")
			return
		}

		dockerChanged := false
		if match, err := project.CompareChecksum(projectDir, "docker-compose.yml", true); err != nil {
			fmt.Printf("Error checking docker-compose.yml checksum: %v\n", err)
			return
		} else if !match {
			dockerChanged = true
			fmt.Println("Warning: checksum mismatch for docker-compose.yml.")

			overwrite := false
			prompt := &survey.Confirm{Message: "Overwrite docker-compose.yml?"}
			if err := survey.AskOne(prompt, &overwrite); err != nil || !overwrite {
				fmt.Println("Aborting migration.")
				return
			}
		}

		packageChanged := false
		if match, err := project.CompareChecksum(projectDir, "package.json", true); err != nil {
			fmt.Printf("Error checking package.json checksum: %v\n", err)
			return
		} else if !match {
			packageChanged = true
			fmt.Println("Warning: checksum mismatch for package.json.")
		}

		fmt.Println("Converting config file.")
		if err := project.ConvertLegacyConfig(projectDir); err != nil {
			fmt.Printf("Error converting project settings: %v\n", err)
			return
		}

		// Reload config so the freshly written jcore.toml (e.g. template) is picked up.
		viper.SetConfigFile(filepath.Join(projectDir, "jcore.toml"))
		_ = viper.MergeInConfig()

		if dockerChanged {
			fmt.Println("Updating docker-compose.yml")
			if err := project.UpdateProject(projectDir, []string{"docker-compose.yml"}, []string{"docker-compose.yml"}); err != nil {
				fmt.Printf("Error updating docker-compose.yml: %v\n", err)
			}
		}

		if err := project.UpdateProject(projectDir, nil, nil); err != nil {
			fmt.Printf("Error updating project: %v\n", err)
			return
		}

		if packageChanged {
			fmt.Println("Adding smol-toml to project.")
			npm := exec.Command("npm", "i", "-D", "smol-toml")
			npm.Dir = projectDir
			npm.Stdout = os.Stdout
			npm.Stderr = os.Stderr
			npm.Stdin = os.Stdin
			if err := npm.Run(); err != nil {
				fmt.Printf("Error installing smol-toml: %v\n", err)
			}
		}

		if err := os.Remove(filepath.Join(projectDir, project.LegacyConfigFilename)); err != nil {
			fmt.Printf("Error removing legacy config file: %v\n", err)
			return
		}

		fmt.Println("Migration complete.")
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
