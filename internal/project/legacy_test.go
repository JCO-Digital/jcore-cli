package project

import (
	"os"
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

	if err := ConvertLegacyConfig(dir); err != nil {
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
