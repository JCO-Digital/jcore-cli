package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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
