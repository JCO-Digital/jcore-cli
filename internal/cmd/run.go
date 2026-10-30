package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JCO-Digital/jcore/internal/docker"
	"github.com/JCO-Digital/jcore/internal/logging"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the WordPress environment",
	Long: `Starts the WordPress development environment for the current project.
It generates the .env file and runs docker compose up.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectDir := currentProjectDir()
		if projectDir == "" || !prepareStart(cmd, projectDir) {
			return
		}
		composeUpProject(cmd, projectDir)
	},
}

// restartCmd represents the restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the current project",
	Long: `Restarts the WordPress development environment for the current project.
It regenerates the project configuration like start does, then stops the
project's containers and brings them back up. Other running projects are
left alone.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectDir := currentProjectDir()
		// Prepare before stopping, so a configuration error leaves the
		// project running instead of taking it down.
		if projectDir == "" || !prepareStart(cmd, projectDir) {
			return
		}

		fmt.Println("Stopping Docker containers...")
		if err := docker.ComposeStop(projectDir); err != nil {
			fmt.Printf("Docker failed: %v\n", err)
			return
		}
		composeUpProject(cmd, projectDir)
	},
}

// currentProjectDir returns the root of the current JCore project, or ""
// (after printing why) when not inside one.
func currentProjectDir() string {
	projectDir, err := project.FindProjectRoot()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return ""
	}
	if projectDir == "" {
		fmt.Println("Error: not in a JCore project (no jcore.toml found)")
	}
	return projectDir
}

// prepareStart runs every step of `start` that comes before docker compose
// up: pre-flight checks, project configuration, conflict checks, the shared
// proxy, dependencies and bind-mounted folders. It reports whether they all
// succeeded, having printed the error otherwise.
func prepareStart(cmd *cobra.Command, projectDir string) bool {
	if !checkFolders() || !checkDocker() {
		fmt.Println("Error: pre-flight checks failed, aborting start. Run 'jcore doctor' for details.")
		return false
	}

	fmt.Println("Finalizing project configuration...")
	if err := project.GenerateEnvFile(projectDir); err != nil {
		fmt.Printf("Error generating .env file: %v\n", err)
		return false
	}
	if err := project.FinalizeProject(projectDir); err != nil {
		fmt.Printf("Error finalizing project: %v\n", err)
		return false
	}

	// Other projects may run alongside this one, behind the shared
	// proxy - unless they'd collide with it.
	if err := project.CheckStartConflicts(projectDir); err != nil {
		fmt.Printf("Error: %v\n", err)
		return false
	}
	if err := ensureProxy(); err != nil {
		fmt.Printf("Error starting the JCore proxy: %v\n", err)
		return false
	}

	forceInstall, _ := cmd.Flags().GetBool("install")
	if err := project.InstallDependencies(projectDir, forceInstall); err != nil {
		fmt.Printf("Error installing dependencies: %v\n", err)
		return false
	}

	// Create every folder docker-compose.yml bind-mounts that doesn't
	// exist yet, before docker itself does — as root, leaving it
	// unwritable by build scripts running as a normal user.
	if err := project.EnsureComposeMountedFolders(projectDir); err != nil {
		fmt.Printf("Error creating compose-mounted folders: %v\n", err)
		return false
	}
	return true
}

// composeUpProject starts the project's containers, attached unless
// --detached or the background mode setting says otherwise, and activates
// the configured theme once they're up.
func composeUpProject(cmd *cobra.Command, projectDir string) {
	detached, _ := cmd.Flags().GetBool("detached")
	if viper.GetString("mode") == "background" {
		detached = true
	}

	// Activate the configured theme once containers are actually up -
	// run concurrently since `docker compose up` blocks in the
	// foreground until stopped. In detached mode, wait for it so it
	// doesn't get killed by the process exiting before it's done.
	themeDone := make(chan struct{})
	go func() {
		project.ActivateTheme(projectDir, viper.GetString("theme"))
		close(themeDone)
	}()

	fmt.Println("Starting Docker containers...")
	if !detached {
		printProjectURLs()
	}
	if err := docker.ComposeUp(projectDir, detached); err != nil {
		fmt.Printf("Docker failed: %v\n", err)
		return
	}
	if detached {
		if viper.GetString("theme") != "" {
			fmt.Println("Waiting for WordPress to finish starting...")
		}
		<-themeDone
		printProjectURLs()
	}
}

// stopCmd represents the stop command. Inside a project it stops that
// project; --all stops every running project. Outside a project it asks
// which one to stop. The shared proxy keeps running either way (see
// `jcore proxy stop`).
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current project (or --all)",
	Long: `Stops the current project. With --all, stops every running JCore project.
Outside a project, asks which running project to stop.
The shared proxy keeps running; stop it with "jcore proxy stop".`,
	Run: func(cmd *cobra.Command, args []string) {
		all, _ := cmd.Flags().GetBool("all")

		if !all {
			projectDir, err := project.FindProjectRoot()
			if err == nil && projectDir != "" {
				if err := docker.ComposeStop(projectDir); err != nil {
					fmt.Printf("Docker failed: %v\n", err)
				}
				return
			}
		}

		running, err := runningProjects()
		if err != nil {
			fmt.Printf("Error checking running projects: %v\n", err)
			return
		}
		if len(running) == 0 {
			fmt.Println("No running projects.")
			return
		}

		if !all {
			running, err = pickProjectsToStop(running)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
		}

		for _, p := range running {
			fmt.Printf("Stopping %s.\n", p.Name)
			if err := docker.ComposeStop(p.Path); err != nil {
				fmt.Printf("Docker failed: %v\n", err)
			}
		}
	},
}

// pickProjectsToStop asks which of the running projects to stop.
func pickProjectsToStop(running []project.DockerProject) ([]project.DockerProject, error) {
	const allOption = "All running projects"
	options := []string{allOption}
	for _, p := range running {
		options = append(options, p.Name)
	}

	choice := ""
	prompt := &survey.Select{Message: "Which project do you want to stop?", Options: options}
	if err := survey.AskOne(prompt, &choice); err != nil {
		return nil, err
	}
	if choice == allOption {
		return running, nil
	}
	for _, p := range running {
		if p.Name == choice {
			return []project.DockerProject{p}, nil
		}
	}
	return nil, nil
}

// runningProjects returns every currently running JCore project.
func runningProjects() ([]project.DockerProject, error) {
	all, err := project.ListDockerProjects()
	if err != nil {
		return nil, err
	}

	var running []project.DockerProject
	for _, p := range all {
		if p.Running {
			running = append(running, p)
		}
	}
	return running, nil
}

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "pull [db|plugins|media|themes|all]",
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
					targets["themes"] = true
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

		legacy, _ := cmd.Flags().GetBool("legacy")

		if targets["plugins"] {
			fmt.Println("Pulling plugins...")
			if err := project.SyncPlugins(projectDir, legacy); err != nil {
				fmt.Printf("Error syncing plugins: %v\n", err)
			}
		}

		if targets["themes"] {
			fmt.Println("Pulling themes...")
			if err := project.SyncThemes(projectDir, legacy); err != nil {
				fmt.Printf("Error syncing themes: %v\n", err)
			}
		}

		if targets["db"] {
			fmt.Println("Pulling database...")
			if err := project.ImportDatabase(projectDir, legacy); err != nil {
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
			if err := project.SyncMedia(projectDir, legacy); err != nil {
				fmt.Printf("Error syncing media: %v\n", err)
			}
		}
	},
}

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [--xdebug] [--notices] <command> [args...]",
	Short: "Run a command in the wordpress container",
	Long: `Runs a command in the project's wordpress container, as its normal user
in the WordPress root, so wp-cli needs neither --allow-root nor --path:

  jcore run wp option get siteurl
  jcore run wp plugin list --format=json

Everything after the command is passed to it unchanged, flags included.
A single quoted argument containing spaces is run through sh -c, so
pipes and && work inside the container: jcore run "wp plugin list | wc -l"

Built for scripts as much as for typing:
  - Xdebug is off (XDEBUG_MODE=off). --xdebug keeps the project's own
    Xdebug setting; XDEBUG_MODE set in your shell is passed on as is.
  - PHP deprecations and notices are hidden and the remaining PHP errors
    go to stderr only, so stdout holds just the command's output. --notices
    shows them. This only affects the command run, not the site.
  - A terminal is allocated only when stdin, stdout and stderr all are
    one; otherwise stdout and stderr stay separate streams.
  - jcore exits with the command's exit code.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Fprintln(os.Stderr, "Error: not in a JCore project (no jcore.toml found)")
			os.Exit(1)
		}

		xdebug, _ := cmd.Flags().GetBool("xdebug")
		notices, _ := cmd.Flags().GetBool("notices")
		opts := docker.ExecOptions{
			TTY: docker.StdioIsTerminal(),
			Env: runEnv(xdebug, os.LookupEnv),
		}
		// Without a terminal wp-cli assumes its output is piped and drops
		// tables and colours; keep them while a person is reading it.
		if !opts.TTY && term.IsTerminal(int(os.Stdout.Fd())) {
			opts.Env = append(opts.Env, "SHELL_PIPE=0")
		}
		if !notices {
			scanDir, err := project.WriteQuietPHPConfig(projectDir)
			if err != nil {
				logging.Warn("Could not write the PHP settings that hide deprecations: %v", err)
			} else {
				opts.Env = append(opts.Env, scanDir)
			}
		}

		if err := docker.ComposeRun(projectDir, "wordpress", opts, runCommandParts(args)); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// runEnv returns the environment jcore run sets for the command: Xdebug
// off, unless keepXdebug asks for the project's own setting or the
// XDEBUG_MODE found through lookupEnv (jcore's own environment) is
// passed on instead.
func runEnv(keepXdebug bool, lookupEnv func(string) (string, bool)) []string {
	if keepXdebug {
		return nil
	}
	if mode, ok := lookupEnv("XDEBUG_MODE"); ok {
		return []string{"XDEBUG_MODE=" + mode}
	}
	return []string{"XDEBUG_MODE=off"}
}

// runCommandParts turns jcore run's arguments into the command to execute.
// A single argument containing whitespace is the documented
// `jcore run "wp plugin list"` form, run through a shell; anything else is
// executed as is.
func runCommandParts(args []string) []string {
	if len(args) == 1 && strings.ContainsAny(args[0], " \t\n") {
		return []string{"sh", "-c", args[0]}
	}
	return args
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
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(attachCmd)

	startCmd.Flags().Bool("detached", false, "Run containers in background")
	startCmd.Flags().BoolP("install", "i", false, "Force reinstalling dependencies even if the install setting is disabled")
	startCmd.Flags().BoolP("force", "f", false, "No longer needed: projects run side by side behind the shared proxy")
	_ = startCmd.Flags().MarkDeprecated("force", "projects now run side by side behind the shared proxy")
	restartCmd.Flags().Bool("detached", false, "Run containers in background")
	restartCmd.Flags().BoolP("install", "i", false, "Force reinstalling dependencies even if the install setting is disabled")
	stopCmd.Flags().BoolP("all", "a", false, "Stop every running JCore project")
	pullCmd.Flags().String("dbfile", "", "Specific database file to import")
	pullCmd.Flags().Bool("legacy", false, "Use legacy in-container import script")
	// Stop at the command, so its own flags (wp ... --format=json) are
	// passed to it instead of being parsed as jcore's.
	runCmd.Flags().SetInterspersed(false)
	runCmd.Flags().Bool("xdebug", false, "Keep the project's Xdebug setting instead of turning Xdebug off")
	runCmd.Flags().Bool("notices", false, "Show PHP deprecations and notices")
}
