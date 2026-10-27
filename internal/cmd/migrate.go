package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

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
then updates project files from the current templates.

jcore generates the project's compose file itself, so an unmodified
docker-compose.yml is removed. A modified one is only removed if you
confirm; otherwise jcore keeps using it until its changes are moved to
compose.override.yml.`,
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

		template, ok := selectMigrateTemplate(cmd, projectDir)
		if !ok {
			return
		}

		removeCompose, ok := confirmLegacyComposeRemoval(projectDir)
		if !ok {
			return
		}

		packageChanged := false
		if match, err := legacyFileMatches(projectDir, "package.json"); err != nil {
			fmt.Printf("Error checking package.json checksum: %v\n", err)
			return
		} else if !match {
			packageChanged = true
			fmt.Println("Warning: checksum mismatch for package.json.")
		}

		fmt.Println("Converting config file.")
		if err := project.ConvertLegacyConfig(projectDir, template); err != nil {
			fmt.Printf("Error converting project settings: %v\n", err)
			return
		}

		// Reload config so the freshly written jcore.toml (e.g. template) is picked up.
		viper.SetConfigFile(filepath.Join(projectDir, "jcore.toml"))
		_ = viper.MergeInConfig()

		if removeCompose {
			fmt.Println("Removing docker-compose.yml, jcore generates compose.jcore.yml instead.")
			if err := os.Remove(filepath.Join(projectDir, project.LegacyComposeFile)); err != nil {
				fmt.Printf("Error removing docker-compose.yml: %v\n", err)
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

// selectMigrateTemplate returns the template to migrate to: the --template
// flag if given, otherwise the user's pick from the catalog, defaulting to
// the template detected from the project's committed theme submodule.
func selectMigrateTemplate(cmd *cobra.Command, projectDir string) (string, bool) {
	catalog, err := project.LoadTemplateCatalog()
	if err != nil {
		fmt.Printf("Error loading template catalog: %v\n", err)
		return "", false
	}
	templateNames := make([]string, 0, len(catalog))
	for k := range catalog {
		templateNames = append(templateNames, k)
	}
	sort.Strings(templateNames)

	template, _ := cmd.Flags().GetString("template")
	if !cmd.Flags().Changed("template") {
		if err := survey.AskOne(&survey.Select{
			Message: "Select the template this project uses:",
			Options: templateNames,
			Default: project.DetectLegacyTemplate(projectDir),
		}, &template); err != nil {
			fmt.Println(err.Error())
			return "", false
		}
	}

	if _, ok := catalog[template]; !ok {
		fmt.Printf("Error: unknown template %q.\n", template)
		return "", false
	}
	return template, true
}

// confirmLegacyComposeRemoval reports whether migrating should remove the
// project's docker-compose.yml: always if it's unmodified, and if the user
// confirms when it has local changes. ok is false if migrating should stop.
func confirmLegacyComposeRemoval(projectDir string) (remove, ok bool) {
	if _, err := os.Stat(filepath.Join(projectDir, project.LegacyComposeFile)); os.IsNotExist(err) {
		return false, true
	}
	match, err := legacyFileMatches(projectDir, project.LegacyComposeFile)
	if err != nil {
		fmt.Printf("Error checking docker-compose.yml checksum: %v\n", err)
		return false, false
	}
	if match {
		return true, true
	}

	fmt.Println("docker-compose.yml has local changes (checksum mismatch). jcore now generates its own")
	fmt.Println("compose file; project-specific changes belong in compose.override.yml.")
	prompt := &survey.Confirm{Message: "Delete docker-compose.yml? (No keeps using it until it's moved to an override)"}
	if err := survey.AskOne(prompt, &remove); err != nil {
		fmt.Println(err.Error())
		return false, false
	}
	return remove, true
}

// legacyFileMatches reports whether relPath matches its stored checksum. A
// file that doesn't exist yet counts as a match: there are no local changes
// to protect, and the template update will simply create it.
func legacyFileMatches(projectDir, relPath string) (bool, error) {
	if _, err := os.Stat(filepath.Join(projectDir, relPath)); os.IsNotExist(err) {
		return true, nil
	}
	return project.CompareChecksum(projectDir, relPath, true)
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().StringP("template", "t", "", "Project template to migrate to (prompted for if omitted)")
}
