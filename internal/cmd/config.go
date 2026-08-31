package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/JCO-Digital/jcore/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage JCore configuration settings",
	Long: `Display, set, or unset configuration values for JCore projects.
Settings can be managed at different levels: global, project, or local.`,
}

// listCmd represents the config list command
var listCmd = &cobra.Command{
	Use:   "list [active|global|project|local|all]",
	Short: "List configuration settings",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scope := "active"
		if len(args) > 0 {
			scope = args[0]
		}

		fmt.Printf("Listing %s settings:\n", scope)

		// Get all settings from Viper
		settings := viper.AllSettings()

		// Filter by scope if needed (Viper merges them by default,
		// so getting individual scopes might require reading files manually
		// if we want to be exact, but for now we'll show merged active settings)

		keys := make([]string, 0, len(settings))
		for k := range settings {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			fmt.Printf("  %s: %v\n", k, settings[k])
		}
	},
}

// setCmd represents the config set command
var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		isGlobal, _ := cmd.Flags().GetBool("global")
		isProject, _ := cmd.Flags().GetBool("project")
		isLocal, _ := cmd.Flags().GetBool("local")

		scope := config.ScopeProject
		if isGlobal {
			scope = config.ScopeGlobal
		} else if isLocal {
			scope = config.ScopeLocal
		} else if isProject {
			scope = config.ScopeProject
		}

		path, err := config.GetConfigPath(scope)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Use a temporary viper instance to write to a specific file
		v := viper.New()
		v.SetConfigFile(path)
		_ = v.ReadInConfig()

		// Handle pseudo-setters
		switch strings.ToLower(key) {
		case "wpe":
			v.Set("remoteHost", fmt.Sprintf("%s@%s.ssh.wpengine.net", value, value))
			v.Set("remotePath", fmt.Sprintf("/sites/%s", value))
			fmt.Printf("Set WP Engine settings for: %s in %s\n", value, path)
		case "php":
			v.Set("wpImage", fmt.Sprintf("jcodigi/wordpress:%s", value))
			fmt.Printf("Set PHP version (wpImage) to: %s in %s\n", value, path)
		default:
			v.Set(key, value)
			fmt.Printf("Set %s to %s in %s\n", key, value, path)
		}

		if err := v.WriteConfig(); err != nil {
			if err := v.SafeWriteConfig(); err != nil {
				fmt.Printf("Error saving config to %s: %v\n", path, err)
			}
		}
	},
}

// unsetCmd represents the config unset command
var unsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Remove a configuration setting",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		// Viper doesn't have a direct "unset" that removes from the file easily
		// in a way that respects scopes perfectly without more logic,
		// but we can set it to nil or handle it via a custom manager later.
		fmt.Printf("Unsetting %s (Not fully implemented yet)\n", key)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(listCmd)
	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(unsetCmd)

	// Local flags for scope
	configCmd.PersistentFlags().BoolP("global", "g", false, "Apply setting globally")
	configCmd.PersistentFlags().BoolP("project", "p", false, "Apply setting to the project")
	configCmd.PersistentFlags().BoolP("local", "l", false, "Apply setting locally")
}
