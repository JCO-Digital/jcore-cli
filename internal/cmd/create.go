package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JCO-Digital/jcore/internal/docker"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an item from template",
	Long:  `Create a new WordPress user, Gutenberg block, or other project components.`,
}

// createUserCmd represents the create user command
var createUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Create a new WordPress user",
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		var qs = []*survey.Question{
			{
				Name: "username",
				Prompt: &survey.Input{
					Message: "Enter username:",
				},
				Validate: survey.Required,
			},
			{
				Name: "email",
				Prompt: &survey.Input{
					Message: "Enter email:",
				},
				Validate: survey.Required,
			},
			{
				Name: "role",
				Prompt: &survey.Select{
					Message: "Select user role:",
					Options: []string{"administrator", "editor", "author", "contributor", "subscriber"},
					Default: "administrator",
				},
			},
			{
				Name: "password",
				Prompt: &survey.Password{
					Message: "Enter password (leave empty to generate):",
				},
			},
		}

		answers := struct {
			Username string
			Email    string
			Role     string
			Password string
		}{}

		err = survey.Ask(qs, &answers)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		wpCmd := []string{"wp", "user", "create", answers.Username, answers.Email, "--role=" + answers.Role}
		if answers.Password != "" {
			wpCmd = append(wpCmd, "--user_pass="+answers.Password)
		}

		fmt.Printf("Creating user %s...\n", answers.Username)
		err = docker.ComposeExec(projectDir, "wordpress", wpCmd)
		if err != nil {
			fmt.Printf("Error creating user: %v\n", err)
		} else {
			fmt.Println("User created successfully.")
		}
	},
}

// createBlockCmd represents the create block command
var createBlockCmd = &cobra.Command{
	Use:   "block",
	Short: "Create a new Gutenberg block",
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		if !project.LohkoInstalled(projectDir) && !promptInstallLohko(projectDir) {
			return
		}
		lohkoSrcDir := filepath.Join(projectDir, "wp-content", "plugins", "lohko", "src")

		templates, err := project.LohkoBlockTemplateNames()
		if err != nil || len(templates) == 0 {
			fmt.Println("Error: no Lohko block templates available.")
			return
		}

		var answers = struct {
			Name        string
			Template    string
			Description string
		}{}

		var qs = []*survey.Question{
			{
				Name: "name",
				Prompt: &survey.Input{
					Message: "Enter block name:",
				},
				Validate: survey.Required,
			},
			{
				Name: "template",
				Prompt: &survey.Select{
					Message: "Select a block template:",
					Options: templates,
					Default: templates[0],
				},
			},
			{
				Name: "description",
				Prompt: &survey.Input{
					Message: "Enter block description (optional):",
				},
			},
		}

		err = survey.Ask(qs, &answers)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		slug := project.Slugify(answers.Name)
		destDir := filepath.Join(lohkoSrcDir, slug)

		if _, err := os.Stat(destDir); err == nil {
			fmt.Printf("Error: Block directory %s already exists.\n", destDir)
			return
		}

		fmt.Printf("Creating block %s (%s) from the %s template...\n", answers.Name, slug, answers.Template)
		if err := project.CreateBlock(destDir, answers.Template, answers.Name, slug, answers.Description); err != nil {
			fmt.Printf("Error creating block: %v\n", err)
			return
		}

		fmt.Println("Block created successfully at:", destDir)
	},
}

// promptInstallLohko asks whether to install the Lohko plugin (not present
// in projectDir), and if confirmed, downloads it and offers to prune its
// bundled example blocks. Returns whether it's now safe to proceed with
// creating a block.
func promptInstallLohko(projectDir string) bool {
	install := false
	confirmPrompt := &survey.Confirm{
		Message: "Lohko is not installed, do you want to install it?",
		Default: true,
	}
	if err := survey.AskOne(confirmPrompt, &install); err != nil {
		fmt.Println(err.Error())
		return false
	}
	if !install {
		fmt.Println("Lohko will not be installed, aborting.")
		return false
	}

	fmt.Println("Installing Lohko...")
	if err := project.InstallLohko(projectDir); err != nil {
		fmt.Printf("Error installing Lohko: %v\n", err)
		return false
	}

	bundled, err := project.LohkoBundledBlocks(projectDir)
	if err != nil || len(bundled) == 0 {
		fmt.Println("Lohko installed successfully.")
		return true
	}

	options := make([]string, len(bundled))
	for i, b := range bundled {
		options[i] = b.Title
	}

	var keep []string
	keepPrompt := &survey.MultiSelect{
		Message: "Select Lohko example blocks to keep (unselected ones are removed):",
		Options: options,
	}
	if err := survey.AskOne(keepPrompt, &keep); err != nil {
		fmt.Println(err.Error())
		return true // Lohko itself is already installed; just skip pruning.
	}

	keepSet := make(map[string]bool, len(keep))
	for _, k := range keep {
		keepSet[k] = true
	}
	for _, b := range bundled {
		if !keepSet[b.Title] {
			_ = project.RemoveLohkoBlock(projectDir, b.Folder)
		}
	}

	fmt.Println("Lohko installed successfully.")
	return true
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.AddCommand(createUserCmd)
	createCmd.AddCommand(createBlockCmd)
}
