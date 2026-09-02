package cmd

import (
	"testing"

	"github.com/JCO-Digital/jcore/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// resetRootLogFlags restores rootCmd's -v/-d/-q/--loglevel persistent
// flags to their defaults, and returns a func to call via defer — these
// are shared global state (the same flags every subcommand inherits), so
// tests that set them must clean up after themselves.
func resetRootLogFlags(t *testing.T) func() {
	t.Helper()
	flags := rootCmd.PersistentFlags()
	return func() {
		_ = flags.Set("verbose", "false")
		_ = flags.Set("debug", "false")
		_ = flags.Set("quiet", "false")
		_ = flags.Set("loglevel", "-1")
	}
}

// fakeCmdWithLetMeBreakThings builds a minimal cobra.Command tree rooted at
// name (with the given parent, if any) carrying its own local
// "letmebreakthings" flag, standing in for the real inherited persistent
// flag without requiring full cobra flag-parsing/merging machinery.
func fakeCmdWithLetMeBreakThings(name string, parent *cobra.Command) *cobra.Command {
	c := &cobra.Command{Use: name}
	c.Flags().Bool("letmebreakthings", false, "")
	if parent != nil {
		parent.AddCommand(c)
	}
	return c
}

func TestRefusePluginInstallComposer_BlocksByDefault(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("pluginInstall", "composer")

	cmd := fakeCmdWithLetMeBreakThings("start", nil)
	if err := refusePluginInstallComposer(cmd); err == nil {
		t.Fatal("expected an error while pluginInstall is composer, got nil")
	}
}

func TestRefusePluginInstallComposer_AllowsWithFlag(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("pluginInstall", "composer")

	cmd := fakeCmdWithLetMeBreakThings("start", nil)
	if err := cmd.Flags().Set("letmebreakthings", "true"); err != nil {
		t.Fatal(err)
	}
	if err := refusePluginInstallComposer(cmd); err != nil {
		t.Fatalf("expected --letmebreakthings to allow it through, got error: %v", err)
	}
}

func TestRefusePluginInstallComposer_AllowsNonComposerValues(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	for _, value := range []string{"remote", "local", ""} {
		viper.Set("pluginInstall", value)
		cmd := fakeCmdWithLetMeBreakThings("start", nil)
		if err := refusePluginInstallComposer(cmd); err != nil {
			t.Fatalf("pluginInstall=%q: unexpected error: %v", value, err)
		}
	}
}

func TestRefusePluginInstallComposer_ExemptsCompletionAndConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("pluginInstall", "composer")

	for _, parentName := range []string{"completion", "config"} {
		parent := fakeCmdWithLetMeBreakThings(parentName, nil)
		child := fakeCmdWithLetMeBreakThings("bash", parent)
		if err := refusePluginInstallComposer(child); err != nil {
			t.Fatalf("%s child command should be exempt, got error: %v", parentName, err)
		}
	}
}

func TestResolveLogLevelFromFlags_NoneSetReturnsSentinel(t *testing.T) {
	defer resetRootLogFlags(t)()

	if got := resolveLogLevelFromFlags(); got != -1 {
		t.Fatalf("resolveLogLevelFromFlags() = %d, want -1 (no flags passed)", got)
	}
}

func TestResolveLogLevelFromFlags_Precedence(t *testing.T) {
	flags := rootCmd.PersistentFlags()

	cases := []struct {
		name string
		set  func()
		want int
	}{
		{"debug", func() { _ = flags.Set("debug", "true") }, logging.LevelDebug},
		{"verbose", func() { _ = flags.Set("verbose", "true") }, logging.LevelVerbose},
		{"quiet", func() { _ = flags.Set("quiet", "true") }, logging.LevelError},
		{"loglevel", func() { _ = flags.Set("loglevel", "3") }, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer resetRootLogFlags(t)()
			c.set()
			if got := resolveLogLevelFromFlags(); got != c.want {
				t.Fatalf("resolveLogLevelFromFlags() with --%s = %d, want %d", c.name, got, c.want)
			}
		})
	}
}

func TestResolveLogLevelFromFlags_DebugWinsOverVerbose(t *testing.T) {
	defer resetRootLogFlags(t)()
	flags := rootCmd.PersistentFlags()
	_ = flags.Set("verbose", "true")
	_ = flags.Set("debug", "true")

	if got := resolveLogLevelFromFlags(); got != logging.LevelDebug {
		t.Fatalf("resolveLogLevelFromFlags() with both --verbose and --debug = %d, want %d (debug wins)", got, logging.LevelDebug)
	}
}
