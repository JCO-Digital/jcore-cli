package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestPullImages_AtMostOncePerIntervalUnlessForced(t *testing.T) {
	pulls := stubComposePull(t)
	dir := t.TempDir()

	if err := pullImages(dir, false); err != nil {
		t.Fatalf("first pull error = %v", err)
	}
	if err := pullImages(dir, false); err != nil {
		t.Fatalf("second pull error = %v", err)
	}
	if len(*pulls) != 1 {
		t.Fatalf("expected one pull within the interval, got %d", len(*pulls))
	}

	// `start --install`, clone and init always pull.
	if err := pullImages(dir, true); err != nil {
		t.Fatalf("forced pull error = %v", err)
	}
	if len(*pulls) != 2 {
		t.Fatalf("expected a forced pull despite the fresh stamp, got %d pulls", len(*pulls))
	}

	// A stamp older than the interval pulls again.
	old := time.Now().Add(-ImagePullInterval - time.Hour)
	if err := os.Chtimes(filepath.Join(dir, imagePullStampRelPath), old, old); err != nil {
		t.Fatal(err)
	}
	if err := pullImages(dir, false); err != nil {
		t.Fatalf("pull after interval error = %v", err)
	}
	if len(*pulls) != 3 {
		t.Fatalf("expected a pull once the stamp is stale, got %d pulls", len(*pulls))
	}
}

func TestPullImages_FailedScheduledPullOnlyWarns(t *testing.T) {
	orig := composePull
	composePull = func(string) error { return errors.New("offline") }
	t.Cleanup(func() { composePull = orig })
	dir := t.TempDir()

	if err := pullImages(dir, false); err != nil {
		t.Fatalf("scheduled pull should only warn, got error %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, imagePullStampRelPath)); err == nil {
		t.Fatal("a failed pull mustn't record a stamp, so the next start tries again")
	}

	if err := pullImages(dir, true); err == nil {
		t.Fatal("a forced pull should return the error")
	}
}
