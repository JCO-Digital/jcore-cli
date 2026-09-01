package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/JCO-Digital/jcore/container"
	"github.com/JCO-Digital/jcore/internal/config"
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

		// Look up the template's catalog entry (branch/themeUrl), if any —
		// unknown templates (e.g. a custom one passed via --template) just
		// skip the branch/theme steps below, rather than erroring.
		var catalogEntry project.TemplateCatalogEntry
		if catalog, err := project.LoadTemplateCatalog(); err == nil {
			catalogEntry = catalog[template]
		}

		branch, _ := cmd.Flags().GetString("branch")
		if branch == "" {
			branch = catalogEntry.Branch
		}
		if branch != "" {
			viper.Set("branch", branch)
		}

		if err := project.ScaffoldProject(targetDir, template); err != nil {
			fmt.Printf("Error during scaffolding: %v\n", err)
			return
		}

		// Initialize git (before the theme download, so its own git-init-ed
		// contents don't get swallowed by a later `git add -A`, mirroring
		// the legacy CLI's ordering).
		cmdGit := exec.Command("git", "init")
		cmdGit.Dir = targetDir
		_ = cmdGit.Run()

		theme := ""
		notheme, _ := cmd.Flags().GetBool("notheme")
		if catalogEntry.ThemeURL != "" && !notheme {
			fmt.Println("Creating theme...")
			theme, err = project.CreateTheme(targetDir, name, catalogEntry.ThemeURL, branch)
			if err != nil {
				fmt.Printf("Warning: failed to create theme: %v\n", err)
				theme = ""
			} else {
				viper.Set("theme", theme)
			}
		}

		// Write jcore.toml: merges with anything already there rather than
		// overwriting, so re-running init in an existing project is safe.
		store, err := config.OpenStore(config.ScopeProject, targetDir, "")
		if err != nil {
			fmt.Printf("Error opening project config: %v\n", err)
			return
		}
		_ = store.Set("projectName", name)
		if branch != "" {
			_ = store.Set("branch", branch)
		}
		if theme != "" {
			_ = store.Set("theme", theme)
		}
		if err := store.Save(); err != nil {
			fmt.Printf("Error writing project config: %v\n", err)
			return
		}

		fmt.Println("Project successfully initialized.")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringP("template", "t", "jcore3", "Project template to use")
	initCmd.Flags().StringP("branch", "b", "", "Git branch of the theme/plugins to track (defaults to the template's own default branch)")
	initCmd.Flags().BoolP("notheme", "n", false, "Skip downloading and creating the child theme")
}
