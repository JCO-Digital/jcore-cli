package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestGenerateEnvFile_ReplaceAlwaysIncludesDefaultDomainRow reproduces the
// legacy TypeScript CLI's createEnv(), which always added a
// "//remoteDomain|//localDomain" row to REPLACE so `jcore pull db` rewrites
// the remote domain to the local one, even with no explicit `replace`
// setting configured.
func TestGenerateEnvFile_ReplaceAlwaysIncludesDefaultDomainRow(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("remoteDomain", "example.com")
	viper.Set("localDomain", "example.test")

	dir := t.TempDir()
	if err := GenerateEnvFile(dir); err != nil {
		t.Fatalf("GenerateEnvFile error = %v", err)
	}

	replace := readEnvVar(t, dir, "REPLACE")
	want := "//example.com|//example.test"
	if replace != want {
		t.Errorf("REPLACE = %q, want %q", replace, want)
	}
}

// TestGenerateEnvFile_ReplacePreservesUserRowsAndDedupes checks the default
// row is added alongside the user's own `replace` rows, without duplicating
// it if the user already included the same row themselves.
func TestGenerateEnvFile_ReplacePreservesUserRowsAndDedupes(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("remoteDomain", "example.com")
	viper.Set("localDomain", "example.test")
	viper.Set("replace", []string{"//old.example.com|//old.example.test"})

	dir := t.TempDir()
	if err := GenerateEnvFile(dir); err != nil {
		t.Fatalf("GenerateEnvFile error = %v", err)
	}

	replace := readEnvVar(t, dir, "REPLACE")
	want := "//example.com|//example.test //old.example.com|//old.example.test"
	if replace != want {
		t.Errorf("REPLACE = %q, want %q", replace, want)
	}

	// Now with the default row already present among the user's rows: it
	// must not be duplicated.
	viper.Set("replace", []string{"//example.com|//example.test", "//old.example.com|//old.example.test"})
	if err := GenerateEnvFile(dir); err != nil {
		t.Fatalf("GenerateEnvFile error = %v", err)
	}
	replace = readEnvVar(t, dir, "REPLACE")
	want = "//example.com|//example.test //old.example.com|//old.example.test"
	if replace != want {
		t.Errorf("REPLACE = %q, want %q (default row deduplicated)", replace, want)
	}
}

// readEnvVar reads and returns the unquoted value of key from the .env file
// GenerateEnvFile wrote in dir.
func readEnvVar(t *testing.T, dir, key string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if v, ok := strings.CutPrefix(line, key+"="); ok {
			return strings.Trim(v, "\"")
		}
	}
	t.Fatalf("%s not found in .env:\n%s", key, raw)
	return ""
}
