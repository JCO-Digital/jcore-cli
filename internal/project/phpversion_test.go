package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// stubPHPPatches is what the stubbed php.net lookup returns in tests.
var stubPHPPatches = map[string]string{
	"8.2": "8.2.99",
	"8.3": "8.3.99",
	"8.4": "8.4.99",
}

// TestMain keeps every test in this package off the network: scaffolding
// and updating resolve the PHP version through latestPHPPatch.
func TestMain(m *testing.M) {
	latestPHPPatch = func(minor string) (string, error) {
		if v, ok := stubPHPPatches[minor]; ok {
			return v, nil
		}
		return "", errors.New("no stub for " + minor)
	}
	os.Exit(m.Run())
}

func withPHPLookupFailing(t *testing.T) {
	t.Helper()
	orig := latestPHPPatch
	latestPHPPatch = func(string) (string, error) { return "", errors.New("offline") }
	t.Cleanup(func() { latestPHPPatch = orig })
}

func writeComposer(t *testing.T, dir, php string) string {
	t.Helper()
	content := "{\n\t\"config\": {\n\t\t\"platform\": {\n\t\t\t\"php\": \"" + php + "\"\n\t\t}\n\t},\n\t\"require\": {\n\t\t\"jco/custom\": \"*\"\n\t}\n}\n"
	path := filepath.Join(dir, "composer.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolvePHPVersion(t *testing.T) {
	tests := []struct {
		name     string
		setting  string
		existing string // composer.json platform php; "" = no composer.json
		template string
		offline  bool
		want     string
	}{
		{name: "new project uses the template's minor", template: "jcore3", want: "8.4.99"},
		{name: "jcore2 template minor", template: "jcore2", want: "8.2.99"},
		{name: "existing minor is kept", existing: "8.3", template: "jcore3", want: "8.3.99"},
		{name: "existing full version moves to latest patch", existing: "8.2.1", template: "jcore3", want: "8.2.99"},
		{name: "setting minor overrides composer.json", setting: "8.4", existing: "8.2.1", template: "jcore2", want: "8.4.99"},
		{name: "setting full version pins", setting: "8.4.10", existing: "8.4", template: "jcore3", want: "8.4.10"},
		{name: "offline keeps the existing full version", existing: "8.3.5", template: "jcore3", offline: true, want: "8.3.5"},
		{name: "offline with bare minor uses built-in patch", existing: "8.4", template: "jcore3", offline: true, want: fallbackPHPPatches["8.4"]},
		{name: "offline with unknown minor falls back to .0", existing: "7.9", template: "jcore3", offline: true, want: "7.9.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			if tt.setting != "" {
				viper.Set("phpVersion", tt.setting)
			}
			if tt.offline {
				withPHPLookupFailing(t)
			}
			dir := t.TempDir()
			if tt.existing != "" {
				writeComposer(t, dir, tt.existing)
			}
			if got := ResolvePHPVersion(dir, tt.template); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSyncComposerPlatformPHP(t *testing.T) {
	dir := t.TempDir()
	path := writeComposer(t, dir, "8.4")
	before, _ := os.ReadFile(path)
	stored, _ := CalculateChecksum(path)

	// Pristine file: value replaced, rest untouched, checksum refreshed.
	checksums := map[string]string{"composer.json": stored}
	changed, err := SyncComposerPlatformPHP(dir, "8.4.99", checksums)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want a change", changed, err)
	}
	after, _ := os.ReadFile(path)
	if want := strings.Replace(string(before), `"php": "8.4"`, `"php": "8.4.99"`, 1); string(after) != want {
		t.Errorf("unexpected content:\n%s\nwant:\n%s", after, want)
	}
	if now, _ := CalculateChecksum(path); checksums["composer.json"] != now {
		t.Error("expected the checksum of a pristine composer.json to be refreshed")
	}

	// Already current: no-op.
	if changed, _ := SyncComposerPlatformPHP(dir, "8.4.99", checksums); changed {
		t.Error("expected no change when the version is already current")
	}

	// Locally modified file: value replaced, checksum left mismatched.
	checksums = map[string]string{"composer.json": "stale"}
	if changed, _ := SyncComposerPlatformPHP(dir, "8.4.100", checksums); !changed {
		t.Error("expected a modified composer.json to still be patched")
	}
	if checksums["composer.json"] != "stale" {
		t.Error("expected a modified composer.json's checksum to stay mismatched")
	}

	// No platform.php: left alone.
	if err := os.WriteFile(path, []byte(`{"require": {}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if changed, _ := SyncComposerPlatformPHP(dir, "8.4.99", checksums); changed {
		t.Error("expected composer.json without platform.php to be left alone")
	}
}

func TestScaffoldAndUpdateTrackPHPPatch(t *testing.T) {
	dir := t.TempDir()
	viper.Reset()
	viper.Set("projectName", "test-project")
	viper.Set("template", "jcore3")
	t.Cleanup(viper.Reset)

	if err := ScaffoldProject(dir, "jcore3"); err != nil {
		t.Fatalf("ScaffoldProject failed: %v", err)
	}
	if got := ComposerPlatformPHP(dir); got != "8.4.99" {
		t.Fatalf("scaffolded platform php = %q, want 8.4.99", got)
	}

	// The project adds its own package, so the checksum no longer matches,
	// and a newer patch is released.
	path := filepath.Join(dir, "composer.json")
	raw, _ := os.ReadFile(path)
	modified := strings.Replace(string(raw), `"require": {`, `"require": {`+"\n\t\t\"jco/custom\": \"*\",", 1)
	if err := os.WriteFile(path, []byte(modified), 0644); err != nil {
		t.Fatal(err)
	}
	stubPHPPatches["8.4"] = "8.4.100"
	t.Cleanup(func() { stubPHPPatches["8.4"] = "8.4.99" })

	if err := UpdateProject(dir, nil, nil); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), `"jco/custom": "*"`) {
		t.Error("expected the project's own composer.json changes to be kept")
	}
	if got := ComposerPlatformPHP(dir); got != "8.4.100" {
		t.Errorf("updated platform php = %q, want 8.4.100", got)
	}
}
