package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JCO-Digital/jcore/internal/config"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/JCO-Digital/jcore/internal/tui"
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
	Use:   "list [active|global|project|local|defaults|all]",
	Short: "List configuration settings",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scope := "active"
		if len(args) > 0 {
			scope = args[0]
		}

		projectRoot, _ := project.FindProjectRoot()
		branch := project.CurrentBranch(projectRoot)

		switch scope {
		case "active":
			printActiveSettings(projectRoot, branch)
		case "global":
			printScope(config.ScopeGlobal, projectRoot, branch, "Global settings")
		case "project":
			printScope(config.ScopeProject, projectRoot, branch, "Project settings")
		case "local":
			printScope(config.ScopeLocal, projectRoot, branch, "Local settings")
		case "defaults":
			printProjectDefaults(projectRoot, branch)
		case "all":
			printActiveSettings(projectRoot, branch)
			if projectRoot != "" {
				printProjectDefaults(projectRoot, branch)
			}
			printScope(config.ScopeGlobal, projectRoot, branch, "Global settings")
			printScope(config.ScopeProject, projectRoot, branch, "Project settings")
			printScope(config.ScopeLocal, projectRoot, branch, "Local settings")
		default:
			fmt.Printf("Unknown scope %q; expected active, global, project, local, defaults, or all.\n", scope)
		}
	},
}

// printActiveSettings prints the fully merged view of every setting viper
// currently has loaded, annotated with which scope (and branch override, if
// any) each value actually resolves from.
func printActiveSettings(projectRoot, branch string) {
	fmt.Println()
	fmt.Println(styleHeading.Render("Active (merged) settings:"))
	settings := viper.AllSettings()
	if len(settings) == 0 {
		fmt.Println(styleDim.Render("  (none)"))
		return
	}

	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("  %s: %s  (%s)\n", styleKey.Render(k), colorizeValue(settings[k]), colorizeScopeLabel(sourceLabel(k, projectRoot, branch)))
	}
}

// sourceLabel renders where a setting's effective value actually comes
// from, e.g. "project", "project@staging", or "default".
func sourceLabel(key, projectRoot, branch string) string {
	res, err := config.Resolve(key, projectRoot, branch)
	if err != nil || res.IsDefault {
		return "default"
	}
	label := strings.ToLower(res.SourceScope.String())
	if res.FromBranchOverride && branch != "" {
		label += "@" + branch
	}
	return label
}

// printProjectDefaults prints the project's own defaults.toml (the
// per-template defaults file scaffolded into every project, distinct from
// jcore.toml) — a resolution layer between Project and Global scope (see
// config.Resolve) that isn't itself a Store `config set`/the TUI can write
// to, so it's read-only here.
func printProjectDefaults(projectRoot, branch string) {
	fmt.Println()
	fmt.Println(styleHeading.Render("Project defaults (defaults.toml) settings:"))
	if projectRoot == "" {
		fmt.Println(styleDim.Render("  (not in a project)"))
		return
	}

	settings := config.ProjectDefaultsFile(projectRoot, branch)
	if len(settings) == 0 {
		fmt.Println(styleDim.Render("  (none)"))
		return
	}

	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("  %s: %s\n", styleKey.Render(k), colorizeValue(settings[k]))
	}
}

// printScope prints exactly what's explicitly set in scope's own file
// (branch-adjusted), annotating each value with "@<branch>" when it
// specifically comes from that file's branch override table rather than its
// top-level settings.
func printScope(scope config.Scope, projectRoot, branch, heading string) {
	fmt.Println()
	fmt.Println(styleHeading.Render(heading + ":"))
	store, err := config.OpenStore(scope, projectRoot, branch)
	if err != nil {
		fmt.Println(styleDim.Render(fmt.Sprintf("  %v", err)))
		return
	}

	settings := store.All()
	if len(settings) == 0 {
		fmt.Println(styleDim.Render("  (none)"))
		return
	}

	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if branch != "" && store.SourceIsBranchOverride(k) {
			fmt.Printf("  %s: %s  (%s)\n", styleKey.Render(k), colorizeValue(settings[k]), styleDim.Render("@"+branch))
		} else {
			fmt.Printf("  %s: %s\n", styleKey.Render(k), colorizeValue(settings[k]))
		}
	}
}

// setCmd represents the config set command
var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		projectRoot, _ := project.FindProjectRoot()
		scope := resolveRequestedScope(cmd, projectRoot)
		branch := project.CurrentBranch(projectRoot)

		if err := config.ValidateScope(key, scope, projectRoot); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		store, err := config.OpenStore(scope, projectRoot, branch)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Pseudo-setters: compound actions that set 1-2 real keys, not real
		// schema keys themselves.
		switch strings.ToLower(key) {
		case "wpe":
			_ = store.Set("remoteHost", fmt.Sprintf("%s@%s.ssh.wpengine.net", value, value))
			_ = store.Set("remotePath", fmt.Sprintf("/sites/%s", value))
			fmt.Printf("Set WP Engine settings for: %s in %s\n", value, store.Path())
		case "php":
			_ = store.Set("wpImage", fmt.Sprintf("jcodigi/wordpress:%s", value))
			fmt.Printf("Set PHP version (wpImage) to: %s in %s\n", value, store.Path())
		default:
			if err := store.Set(key, value); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			fmt.Printf("Set %s to %s in %s\n", key, value, store.Path())
		}

		if err := store.Save(); err != nil {
			fmt.Printf("Error saving config to %s: %v\n", store.Path(), err)
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

		projectRoot, _ := project.FindProjectRoot()
		scope := resolveRequestedScope(cmd, projectRoot)
		branch := project.CurrentBranch(projectRoot)

		if err := config.ValidateScope(key, scope, projectRoot); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		store, err := config.OpenStore(scope, projectRoot, branch)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if _, ok := store.Get(key); !ok {
			fmt.Printf("%s is not set in %s\n", key, store.Path())
			return
		}

		if err := store.Unset(key); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if err := store.Save(); err != nil {
			fmt.Printf("Error saving config to %s: %v\n", store.Path(), err)
			return
		}
		fmt.Printf("Removed %s from %s\n", key, store.Path())
	},
}

// editCmd launches the interactive TUI settings editor.
var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Interactively browse and edit configuration settings",
	Long: `Opens a full-screen editor listing every known jcore setting, its
current effective value, and which scope (default, global, project, or
local) that value actually comes from. Editing a setting prompts for which
scope to save it to when more than one is applicable.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectRoot, _ := project.FindProjectRoot()
		branch := project.CurrentBranch(projectRoot)
		m := tui.NewModel(projectRoot, branch)
		if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
			fmt.Fprintln(os.Stderr, "Error running config editor:", err)
			os.Exit(1)
		}
	},
}

// resolveRequestedScope determines the scope requested via -g/-p/-l flags.
// With none given, it defaults to Project when run inside a project (this
// command's long-standing behavior) or Global otherwise — matching the
// legacy TypeScript CLI's default, rather than hard-failing with "not in a
// jcore project" for a plain `config set` run outside one.
func resolveRequestedScope(cmd *cobra.Command, projectRoot string) config.Scope {
	isGlobal, _ := cmd.Flags().GetBool("global")
	isLocal, _ := cmd.Flags().GetBool("local")

	switch {
	case isGlobal:
		return config.ScopeGlobal
	case isLocal:
		return config.ScopeLocal
	case projectRoot == "":
		return config.ScopeGlobal
	default:
		return config.ScopeProject
	}
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(listCmd)
	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(unsetCmd)
	configCmd.AddCommand(editCmd)

	// Local flags for scope
	configCmd.PersistentFlags().BoolP("global", "g", false, "Apply setting globally")
	configCmd.PersistentFlags().BoolP("project", "p", false, "Apply setting to the project")
	configCmd.PersistentFlags().BoolP("local", "l", false, "Apply setting locally")
}
