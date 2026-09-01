package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// makefileTouchingMarker is a Makefile whose "install" target creates
// marker.txt in the project dir, so tests can assert whether `make install`
// actually ran without depending on npm/pnpm/composer/docker being
// reachable in the test environment.
const makefileTouchingMarker = `install:
	touch marker.txt
`

func TestInstallDependencies_NoopWhenDisabledAndNotForced(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("install", false)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefileTouchingMarker), 0644); err != nil {
		t.Fatal(err)
	}

	if err := InstallDependencies(dir, false); err != nil {
		t.Fatalf("InstallDependencies error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); err == nil {
		t.Fatal("expected 'make install' not to have run while install=false and force=false")
	}
}

func TestInstallDependencies_ForceRunsEvenWhenSettingDisabled(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("install", false)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefileTouchingMarker), 0644); err != nil {
		t.Fatal(err)
	}

	if err := InstallDependencies(dir, true); err != nil {
		t.Fatalf("InstallDependencies error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); err != nil {
		t.Fatal("expected 'make install' to have run with force=true")
	}
}

func TestInstallDependencies_RunsWhenSettingEnabled(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("install", true)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefileTouchingMarker), 0644); err != nil {
		t.Fatal(err)
	}

	if err := InstallDependencies(dir, false); err != nil {
		t.Fatalf("InstallDependencies error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); err != nil {
		t.Fatal("expected 'make install' to have run with install=true")
	}
}

func TestInstallDependencies_MakefileTakesPriorityOverNodeAndComposer(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("install", true)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefileTouchingMarker), 0644); err != nil {
		t.Fatal(err)
	}
	// If present, package.json/composer.json would trigger real npm/pnpm/
	// composer/docker invocations, which aren't reliable in a test
	// sandbox — the Makefile branch must short-circuit before reaching them.
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "composer.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := InstallDependencies(dir, false); err != nil {
		t.Fatalf("InstallDependencies error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); err != nil {
		t.Fatal("expected 'make install' to have run")
	}
}
