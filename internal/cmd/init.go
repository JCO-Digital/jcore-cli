package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/JCO-Digital/jcore/container"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new JCore project",
	Long: `Creates a new JCore project directory with a default configuration and project skeleton.
If no name is provided, it will use the current directory name.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := ""
		targetDir := ""
		var err error

		if len(args) > 0 {
			name = args[0]
			targetDir, err = os.Getwd()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			targetDir = filepath.Join(targetDir, name)
		} else {
			targetDir, err = os.Getwd()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			name = filepath.Base(targetDir)
		}

		template, _ := cmd.Flags().GetString("template")
		fmt.Printf("Initializing JCore project: %s in %s (Template: %s)\n", name, targetDir, template)

		// Merge the template's own defaults (e.g. theme) so the initial scaffold
		// renders with the right values; on later runs jcore.toml/defaults.toml
		// on disk take over via the normal config loading in root.go.
		if defaults, err := container.TemplateAssets.Open(filepath.Join("templates", template, "defaults.toml")); err == nil {
			viper.SetConfigType("toml")
			_ = viper.MergeConfig(defaults)
			defaults.Close()
		}
		viper.Set("projectName", name)

		if err := project.ScaffoldProject(targetDir, template); err != nil {
			fmt.Printf("Error during scaffolding: %v\n", err)
			return
		}

		// Create jcore.toml if it doesn't exist
		jcoreConfig := filepath.Join(targetDir, "jcore.toml")
		if _, err := os.Stat(jcoreConfig); os.IsNotExist(err) {
			configContent := fmt.Sprintf("projectName = \"%s\"\n", name)
			_ = os.WriteFile(jcoreConfig, []byte(configContent), 0644)
		}

		// Initialize git
		cmdGit := exec.Command("git", "init")
		cmdGit.Dir = targetDir
		_ = cmdGit.Run()

		fmt.Println("Project successfully initialized.")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringP("template", "t", "jcore3", "Project template to use")
}
