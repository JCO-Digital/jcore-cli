package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

		lohkoSrcDir := filepath.Join(projectDir, "wp-content", "plugins", "lohko", "src")
		if _, err := os.Stat(lohkoSrcDir); os.IsNotExist(err) {
			fmt.Println("Error: Lohko plugin source directory not found. Is Lohko installed?")
			return
		}

		var answers = struct {
			Name        string
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

		slug := slugify(answers.Name)
		destDir := filepath.Join(lohkoSrcDir, slug)

		if _, err := os.Stat(destDir); err == nil {
			fmt.Printf("Error: Block directory %s already exists.\n", destDir)
			return
		}

		fmt.Printf("Creating block %s (%s)...\n", answers.Name, slug)

		// For now, we'll create a minimal block.json and a placeholder directory.
		// In a full implementation, we would copy from a template.
		if err := os.MkdirAll(destDir, 0755); err != nil {
			fmt.Printf("Error creating block directory: %v\n", err)
			return
		}

		blockData := map[string]interface{}{
			"apiVersion":  2,
			"name":        "jcore/" + slug,
			"title":       answers.Name,
			"category":    "formatting",
			"description": answers.Description,
			"supports": map[string]interface{}{
				"html": false,
			},
			"textdomain": "lohko",
		}

		blockJsonPath := filepath.Join(destDir, "block.json")
		file, _ := json.MarshalIndent(blockData, "", "    ")
		_ = os.WriteFile(blockJsonPath, file, 0644)

		fmt.Println("Block created successfully at:", destDir)
	},
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.AddCommand(createUserCmd)
	createCmd.AddCommand(createBlockCmd)
}
