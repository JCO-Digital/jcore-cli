package cmd

import (
	"os"
	"path/filepath"

	"github.com/JCO-Digital/jcore/internal/container"
	"github.com/JCO-Digital/jcore/internal/project"
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
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// 1. Set Defaults
		defaultConfig, err := container.BaseAssets.Open("base/defaults.toml")
		if err == nil {
			viper.SetConfigType("toml")
			_ = viper.MergeConfig(defaultConfig)
			defaultConfig.Close()
		}

		// 2. Global Config
		home, err := os.UserHomeDir()
		if err == nil {
			configPath := filepath.Join(home, ".config", "jcore")
			viper.AddConfigPath(configPath)
			viper.SetConfigType("toml")
			viper.SetConfigName("config")
			if err := viper.MergeInConfig(); err == nil {
				if viper.GetBool("verbose") || viper.GetBool("debug") {
					os.Stderr.WriteString("Loaded global config from " + configPath + "\n")
				}
			}
		}

		// 3. Project & Local Config
		// Find project root
		projectRoot, _ := project.FindProjectRoot()
		if projectRoot != "" {
			// Project Defaults
			projectDefaults := filepath.Join(projectRoot, "defaults.toml")
			if _, err := os.Stat(projectDefaults); err == nil {
				viper.SetConfigFile(projectDefaults)
				if err := viper.MergeInConfig(); err == nil {
					if viper.GetBool("verbose") || viper.GetBool("debug") {
						os.Stderr.WriteString("Loaded project defaults from " + projectDefaults + "\n")
					}
				}
			}

			// Project Config
			projectConfig := filepath.Join(projectRoot, "jcore.toml")
			if _, err := os.Stat(projectConfig); err == nil {
				viper.SetConfigFile(projectConfig)
				if err := viper.MergeInConfig(); err == nil {
					if viper.GetBool("verbose") || viper.GetBool("debug") {
						os.Stderr.WriteString("Loaded project config from " + projectConfig + "\n")
					}
				}
			}

			// Local Config
			localConfig := filepath.Join(projectRoot, ".localConfig.toml")
			if _, err := os.Stat(localConfig); err == nil {
				viper.SetConfigFile(localConfig)
				if err := viper.MergeInConfig(); err == nil {
					if viper.GetBool("verbose") || viper.GetBool("debug") {
						os.Stderr.WriteString("Loaded local config from " + localConfig + "\n")
					}
				}
			}
		}
	}

	viper.AutomaticEnv()
}
