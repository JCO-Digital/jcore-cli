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

// stubComposePull replaces the Docker image pull for the duration of the
// test and returns a pointer to the project dirs it was called with.
func stubComposePull(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	orig := composePull
	composePull = func(projectDir string) error {
		calls = append(calls, projectDir)
		return nil
	}
	t.Cleanup(func() { composePull = orig })
	return &calls
}

func TestInstallDependencies_NoopWhenDisabledAndNotForced(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	pulls := stubComposePull(t)
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
	if len(*pulls) != 0 {
		t.Fatalf("expected no image pull while install=false and force=false, got %d", len(*pulls))
	}
}

func TestInstallDependencies_ForceRunsEvenWhenSettingDisabled(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	pulls := stubComposePull(t)
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
	// Regression: the Makefile branch used to return before pulling images.
	if len(*pulls) != 1 || (*pulls)[0] != dir {
		t.Fatalf("expected one image pull for %s after 'make install', got %v", dir, *pulls)
	}
}

func TestInstallDependencies_RunsWhenSettingEnabled(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	pulls := stubComposePull(t)
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
	// Regression: the Makefile branch used to return before pulling images.
	if len(*pulls) != 1 || (*pulls)[0] != dir {
		t.Fatalf("expected one image pull for %s after 'make install', got %v", dir, *pulls)
	}
}

func TestInstallDependencies_MakefileTakesPriorityOverNodeAndComposer(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	pulls := stubComposePull(t)
	viper.Set("install", true)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefileTouchingMarker), 0644); err != nil {
		t.Fatal(err)
	}
	// If present, package.json/composer.json would trigger real npm/pnpm/
	// composer invocations, which aren't reliable in a test sandbox — the
	// Makefile branch must take their place.
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
	// Regression: the Makefile branch used to return before pulling images.
	if len(*pulls) != 1 || (*pulls)[0] != dir {
		t.Fatalf("expected one image pull for %s after 'make install', got %v", dir, *pulls)
	}
}
