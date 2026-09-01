package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/AlecAivazis/survey/v2"
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
	Long: `Creates a new JCore project in a new directory named after the
project (a sibling of the current directory). Prompts interactively for
anything not already given via the [name] argument or --template/--branch:
project name, template, and (if the template offers more than one) branch.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if name == "" {
			if err := survey.AskOne(&survey.Input{Message: "Enter a project name:"}, &name); err != nil {
				fmt.Println(err.Error())
				return
			}
		}
		if name == "" {
			fmt.Println("Error: no project name given.")
			return
		}

		catalog, err := project.LoadTemplateCatalog()
		if err != nil {
			fmt.Printf("Error loading template catalog: %v\n", err)
			return
		}
		templateNames := make([]string, 0, len(catalog))
		for k := range catalog {
			templateNames = append(templateNames, k)
		}
		sort.Strings(templateNames)

		template, _ := cmd.Flags().GetString("template")
		if !cmd.Flags().Changed("template") {
			if err := survey.AskOne(&survey.Select{
				Message: "Select a project template:",
				Options: templateNames,
				Default: template,
			}, &template); err != nil {
				fmt.Println(err.Error())
				return
			}
		}

		catalogEntry, ok := catalog[template]
		if !ok {
			fmt.Printf("Error: unknown template %q.\n", template)
			return
		}

		branch, _ := cmd.Flags().GetString("branch")
		if !cmd.Flags().Changed("branch") {
			if len(catalogEntry.Branches) > 1 {
				if err := survey.AskOne(&survey.Select{
					Message: "Select a branch:",
					Options: catalogEntry.Branches,
					Default: catalogEntry.Branch,
				}, &branch); err != nil {
					fmt.Println(err.Error())
					return
				}
			} else {
				branch = catalogEntry.Branch
			}
		}
		if branch == "" {
			fmt.Println("Error: no branch selected.")
			return
		}

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		targetDir := filepath.Join(cwd, project.Slugify(name))
		if _, err := os.Stat(targetDir); err == nil {
			fmt.Printf("Error: %s already exists.\n", targetDir)
			return
		}

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
		viper.Set("branch", branch)

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
		_ = store.Set("branch", branch)
		if theme != "" {
			_ = store.Set("theme", theme)
		}
		if err := store.Save(); err != nil {
			fmt.Printf("Error writing project config: %v\n", err)
			return
		}

		// Commit the initial scaffold before finalizing: FinalizeProject's
		// site.conf/php.ini rendering and InstallDependencies' lockfiles
		// are per-environment output, not part of the project's own
		// history, mirroring the legacy CLI's createProject() ordering.
		addCmd := exec.Command("git", "add", "-A")
		addCmd.Dir = targetDir
		if out, err := addCmd.CombinedOutput(); err != nil {
			fmt.Printf("Warning: git add failed: %v\n%s", err, out)
		} else {
			commitCmd := exec.Command("git", "commit", "-m", "Initial Commit")
			commitCmd.Dir = targetDir
			if out, err := commitCmd.CombinedOutput(); err != nil {
				fmt.Printf("Warning: git commit failed: %v\n%s", err, out)
			}
		}

		if err := project.GenerateEnvFile(targetDir); err != nil {
			fmt.Printf("Warning: failed to generate .env: %v\n", err)
		}
		if err := project.FinalizeProject(targetDir); err != nil {
			fmt.Printf("Warning: failed to finalize project: %v\n", err)
		}
		if err := project.InstallDependencies(targetDir, true); err != nil {
			fmt.Printf("Warning: failed to install dependencies: %v\n", err)
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
