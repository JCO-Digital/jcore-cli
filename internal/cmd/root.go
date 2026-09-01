package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JCO-Digital/jcore/container"
	"github.com/JCO-Digital/jcore/internal/config"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/JCO-Digital/jcore/internal/update"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "jcore",
	Short: "A command-line interface for jcore WordPress development",
	Long: `JCore CLI is a tool designed to manage WordPress development environments.
It simplifies the process of setting up, running, and maintaining WordPress projects using Docker.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := refusePluginInstallComposer(cmd); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Kick off a background check (at most once every update.CheckInterval)
		// and surface the result of the last one. Both are no-ops when
		// JCORE_NO_UPDATE_CHECK is set (e.g. in CI), or for a command in
		// updateNoticeExemptCommands.
		if os.Getenv("JCORE_NO_UPDATE_CHECK") != "" || isUpdateNoticeExempt(cmd) {
			return
		}
		update.MaybeCheckInBackground()
		if notice := update.AvailableNotice(config.AppVersion); notice != "" {
			fmt.Fprintln(os.Stderr, notice)
		}
	},
}

// refusePluginInstallComposer refuses to run when pluginInstall is set to
// the deprecated "composer" value, mirroring the legacy TypeScript CLI's
// identical safety check (composer-managed plugins break the mainWP/wp-cli
// workflow): an error unless overridden with --letmebreakthings.
//
// Exempt: the "completion" command tree (shells run it at startup
// regardless of any project's settings, same reasoning as
// isUpdateNoticeExempt) and the "config" command tree (so `config
// set`/`edit` remain usable to actually fix the setting without also
// needing --letmebreakthings).
func refusePluginInstallComposer(cmd *cobra.Command) error {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "completion" || c.Name() == "config" {
			return nil
		}
	}

	if viper.GetString("pluginInstall") != "composer" {
		return nil
	}

	if letme, _ := cmd.Flags().GetBool("letmebreakthings"); letme {
		return nil
	}

	return fmt.Errorf(`Error: pluginInstall is set to the deprecated "composer" mode, which breaks the mainWP/wp-cli workflow.
Change it to "remote" (recommended) or "local", e.g.:
  jcore config set pluginInstall remote
  jcore config edit
Or pass --letmebreakthings to run this command anyway.`)
}

// isUpdateNoticeExempt reports whether cmd is exempt from the background
// update check and its stderr notice: any "completion" command (update
// self shells out to it internally, repeatedly, on every self-update, to
// regenerate completion scripts), or specifically "jcore config edit"
// (a stray stderr write right before tea.WithAltScreen() takes over the
// terminal would either be invisibly wiped or race with the alt-screen
// escape sequences).
func isUpdateNoticeExempt(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "completion" {
			return true
		}
	}
	return cmd.Name() == "edit" && cmd.Parent() != nil && cmd.Parent().Name() == "config"
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/jcore/config.toml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "debug output")
	rootCmd.PersistentFlags().Bool("letmebreakthings", false, `override the refusal to run while pluginInstall is the deprecated "composer" value`)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		projectRoot, _ := project.FindProjectRoot()
		branch := project.CurrentBranch(projectRoot)

		// 1. Set Defaults
		defaultConfig, err := container.BaseAssets.Open("base/defaults.toml")
		if err == nil {
			viper.SetConfigType("toml")
			_ = viper.MergeConfig(defaultConfig)
			defaultConfig.Close()
		}

		// 2. Global Config
		if globalPath, err := config.GetConfigPath(config.ScopeGlobal, ""); err == nil {
			mergeTOMLWithBranchOverlay(globalPath, branch, "Loaded global config from "+globalPath)
		}

		// 3. Project & Local Config
		if projectRoot != "" {
			// Project Defaults (the per-template defaults.toml scaffolded
			// into every project, distinct from jcore.toml)
			projectDefaults := filepath.Join(projectRoot, "defaults.toml")
			mergeTOMLWithBranchOverlay(projectDefaults, branch, "Loaded project defaults from "+projectDefaults)

			// Project Config
			if projectConfig, err := config.GetConfigPath(config.ScopeProject, projectRoot); err == nil {
				mergeTOMLWithBranchOverlay(projectConfig, branch, "Loaded project config from "+projectConfig)
			}

			// Local Config
			if localConfig, err := config.GetConfigPath(config.ScopeLocal, projectRoot); err == nil {
				mergeTOMLWithBranchOverlay(localConfig, branch, "Loaded local config from "+localConfig)
			}
		}
	}

	viper.AutomaticEnv()
}

// mergeTOMLWithBranchOverlay loads path — applying any "branch-<branch>"
// override table it contains, so a project's jcore.toml (or any other
// scope's file) can vary a setting by the currently checked-out git
// branch — and merges the result into the global viper config at its
// normal, call-order-determined precedence. Missing or effectively empty
// files are silently skipped, matching the previous os.Stat-gated behavior.
func mergeTOMLWithBranchOverlay(path, branch, loadedMsg string) {
	merged, err := config.LoadTOMLWithBranchOverlay(path, branch)
	if err != nil || len(merged) == 0 {
		return
	}
	if err := viper.MergeConfigMap(merged); err == nil {
		if viper.GetBool("verbose") || viper.GetBool("debug") {
			os.Stderr.WriteString(loadedMsg + "\n")
		}
	}
}
