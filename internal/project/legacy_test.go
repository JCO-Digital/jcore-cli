package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

const sampleLegacyConfig = `# Legacy jcore config
NAME=mysite
THEME=mytheme
BRANCH=main
REMOTEHOST=user@example.com
REMOTEPATH=/var/www/mysite
DB_EXCLUDE=(
  "wp_options"
  "wp_users"
)
DOMAINS=(
  "example.com;mysite"
  "www.example.com;mysite-www"
)
INSTALL=true
`

func TestConvertLegacyConfig(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, LegacyConfigFilename), []byte(sampleLegacyConfig), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	if err := ConvertLegacyConfig(dir, "jcore2"); err != nil {
		t.Fatalf("ConvertLegacyConfig failed: %v", err)
	}

	v := viper.New()
	v.SetConfigFile(filepath.Join(dir, "jcore.toml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read generated jcore.toml: %v", err)
	}

	cases := map[string]string{
		"projectname":  "mysite",
		"template":     "jcore2",
		"theme":        "mytheme",
		"branch":       "main",
		"remotehost":   "user@example.com",
		"remotepath":   "/var/www/mysite",
		"remotedomain": "example.com",
		"localdomain":  "mysite.localhost",
	}
	for key, want := range cases {
		if got := v.GetString(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	wantDomains := []string{"mysite.localhost", "mysite-www.localhost"}
	if got := v.GetStringSlice("domains"); !equalSlices(got, wantDomains) {
		t.Errorf("domains = %v, want %v", got, wantDomains)
	}

	wantReplace := []string{"//example.com|//mysite.localhost", "//www.example.com|//mysite-www.localhost"}
	if got := v.GetStringSlice("replace"); !equalSlices(got, wantReplace) {
		t.Errorf("replace = %v, want %v", got, wantReplace)
	}

	wantExclude := []string{"wp_options", "wp_users"}
	if got := v.GetStringSlice("dbexclude"); !equalSlices(got, wantExclude) {
		t.Errorf("dbExclude = %v, want %v", got, wantExclude)
	}

	if !v.GetBool("install") {
		t.Errorf("install = false, want true")
	}

	// Regression check: viper.WriteConfig lower-cases every key, which
	// previously corrupted this function's output. Confirm the raw file
	// still has real camelCase keys, not e.g. "projectname".
	raw, err := os.ReadFile(filepath.Join(dir, "jcore.toml"))
	if err != nil {
		t.Fatalf("failed to read generated jcore.toml: %v", err)
	}
	for _, key := range []string{"projectName", "remoteHost", "remotePath", "remoteDomain", "localDomain"} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("jcore.toml missing correctly-cased key %q; got:\n%s", key, raw)
		}
	}
}

// sampleVagrantLegacyConfig is the oldest, Vagrant-era config.sh format:
// NAME comes from the environment and the remote is described by WPENAME/WEB.
const sampleVagrantLegacyConfig = `#!/usr/bin/env bash
export DEBIAN_FRONTEND=noninteractive

if [ -z "$NAME" ]; then
    NAME="vagrant"
fi

WEBROOT="/var/www/vagrant"
DB="vagrant"
USER=$DB
PASSWORD=$USER
EMAIL="developer@jco.fi"
WPENAME="uppdev"
WEB="$WPENAME.wpengine.com"
SSH=$WPENAME
UBPNAME="https://$WEB"
VAGRANTNAME="$NAME.test"
THEME=$NAME
PLUGINS=(
)
`

func TestConvertLegacyConfigVagrantFormat(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mysite")
	if err := os.MkdirAll(filepath.Join(dir, "wp-content", "themes", "realtheme"), 0755); err != nil {
		t.Fatalf("failed to create theme dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, LegacyConfigFilename), []byte(sampleVagrantLegacyConfig), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	if err := ConvertLegacyConfig(dir, "blank"); err != nil {
		t.Fatalf("ConvertLegacyConfig failed: %v", err)
	}

	v := viper.New()
	v.SetConfigFile(filepath.Join(dir, "jcore.toml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read generated jcore.toml: %v", err)
	}

	cases := map[string]string{
		"projectname":  "mysite",
		"template":     "blank",
		"theme":        "realtheme",
		"remotehost":   "uppdev@uppdev.ssh.wpengine.net",
		"remotepath":   "/sites/uppdev",
		"remotedomain": "uppdev.wpengine.com",
		"localdomain":  "mysite.localhost",
	}
	for key, want := range cases {
		if got := v.GetString(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	if got, want := v.GetStringSlice("domains"), []string{"mysite.localhost"}; !equalSlices(got, want) {
		t.Errorf("domains = %v, want %v", got, want)
	}
	if got, want := v.GetStringSlice("replace"), []string{"//uppdev.wpengine.com|//mysite.localhost"}; !equalSlices(got, want) {
		t.Errorf("replace = %v, want %v", got, want)
	}
}

func TestDetectLegacyTemplate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	// newRepo creates a git repo with the given paths committed as
	// submodules (gitlinks) and the given .gitmodules content.
	newRepo := func(gitmodules string, submodules ...string) string {
		dir := t.TempDir()
		run := func(args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v failed: %v\n%s", args, err, out)
			}
		}
		run("init")
		if gitmodules != "" {
			if err := os.WriteFile(filepath.Join(dir, ".gitmodules"), []byte(gitmodules), 0644); err != nil {
				t.Fatal(err)
			}
			run("add", ".gitmodules")
		}
		for _, path := range submodules {
			run("update-index", "--add", "--cacheinfo", "160000,a39b904810da89e481054189f4c0de6403d7d4a5,"+path)
		}
		return dir
	}

	cases := []struct {
		name string
		dir  string
		want string
	}{
		{"not a git repo", t.TempDir(), "blank"},
		{"no submodules", newRepo(""), "blank"},
		{"jcore2 submodule", newRepo("", "wp-content/themes/jcore2"), "jcore2"},
		{"jcore submodule", newRepo("", "wp-content/themes/jcore"), "jcore1"},
		// Listed in .gitmodules but never committed: not trusted.
		{"stale .gitmodules", newRepo("[submodule \"jcore\"]\n\tpath = wp-content/themes/jcore\n"), "blank"},
	}
	for _, c := range cases {
		if got := DetectLegacyTemplate(c.dir); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
