package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JCO-Digital/jcore/internal/docker"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the WordPress environment",
	Long: `Starts the WordPress development environment for the current project.
It generates the .env file and runs docker compose up.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		fmt.Println("Finalizing project configuration...")
		if err := project.GenerateEnvFile(projectDir); err != nil {
			fmt.Printf("Error generating .env file: %v\n", err)
			return
		}
		if err := project.FinalizeProject(projectDir); err != nil {
			fmt.Printf("Error finalizing project: %v\n", err)
			return
		}

		forceInstall, _ := cmd.Flags().GetBool("install")
		if err := project.InstallDependencies(projectDir, forceInstall); err != nil {
			fmt.Printf("Error installing dependencies: %v\n", err)
			return
		}

		detached, _ := cmd.Flags().GetBool("detached")
		if viper.GetString("mode") == "background" {
			detached = true
		}

		fmt.Println("Starting Docker containers...")
		if err := docker.ComposeUp(projectDir, detached); err != nil {
			fmt.Printf("Docker failed: %v\n", err)
			return
		}
	},
}

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the WordPress environment",
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		fmt.Println("Stopping Docker containers...")
		if err := docker.ComposeStop(projectDir); err != nil {
			fmt.Printf("Docker failed: %v\n", err)
			return
		}
	},
}

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "pull [db|plugins|media|all]",
	Short: "Sync content from upstream",
	Long: `Pulls data from the remote environment to the local environment.
If no target is specified, it defaults to pulling the database and plugins.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		// Pull logic
		targets := make(map[string]bool)
		if len(args) == 0 {
			targets["db"] = true
			targets["plugins"] = true
		} else {
			for _, arg := range args {
				if arg == "all" {
					targets["db"] = true
					targets["plugins"] = true
					targets["media"] = true
				} else {
					targets[arg] = true
				}
			}
		}

		dbFile, _ := cmd.Flags().GetString("dbfile")
		if dbFile != "" {
			fmt.Printf("Using specific database file: %s\n", dbFile)
			// Replicating logic: move selected db file to update.sql
			sqlDir := filepath.Join(projectDir, ".jcore", "sql")
			src := filepath.Join(sqlDir, dbFile)
			dest := filepath.Join(sqlDir, "update.sql")
			if _, err := os.Stat(src); err == nil {
				_ = os.Rename(src, dest)
			} else {
				fmt.Printf("Warning: Database file %s not found in %s\n", dbFile, sqlDir)
			}
		}

		fmt.Println("Finalizing project configuration...")
		if err := project.GenerateEnvFile(projectDir); err != nil {
			fmt.Printf("Error generating .env file: %v\n", err)
			return
		}

		if targets["plugins"] {
			fmt.Println("Pulling plugins...")
			if err := project.SyncPlugins(projectDir); err != nil {
				fmt.Printf("Error syncing plugins: %v\n", err)
			}
		}

		if targets["db"] {
			fmt.Println("Pulling database...")
			if err := project.ImportDatabase(projectDir); err != nil {
				fmt.Printf("Error importing database: %v\n", err)
			}
		}

		if targets["plugins"] || targets["db"] {
			fmt.Println("Installing local plugins...")
			if err := project.InstallLocalPlugins(projectDir); err != nil {
				fmt.Printf("Error installing local plugins: %v\n", err)
			}
		}

		if targets["media"] {
			fmt.Println("Pulling media...")
			if err := project.SyncMedia(projectDir); err != nil {
				fmt.Printf("Error syncing media: %v\n", err)
			}
		}
	},
}

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run <command>",
	Short: "Run a command in the wordpress container",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		_ = docker.ComposeExec(projectDir, "wordpress", args)
	},
}

// attachCmd represents the attach command
var attachCmd = &cobra.Command{
	Use:   "attach [container]",
	Short: "Attach to the logs of the running containers",
	Long: `Attaches to the logs of all containers in the current project.
Optionally pass a container name to attach to a single container's logs.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		service := ""
		if len(args) > 0 {
			service = args[0]
		}

		fmt.Println("Attaching to logs...")
		if err := docker.ComposeLogs(projectDir, service); err != nil {
			fmt.Printf("Docker failed: %v\n", err)
			return
		}
	},
}

// shellCmd represents the shell command
var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Open a shell in the wordpress container",
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		_ = docker.ComposeExec(projectDir, "wordpress", []string{"/bin/bash"})
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(attachCmd)

	startCmd.Flags().Bool("detached", false, "Run containers in background")
	startCmd.Flags().BoolP("install", "i", false, "Force reinstalling dependencies even if the install setting is disabled")
	pullCmd.Flags().String("dbfile", "", "Specific database file to import")
}
