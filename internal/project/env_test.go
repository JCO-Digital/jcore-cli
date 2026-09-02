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

// TestGenerateEnvFile_DomainDefaults reproduces a project whose jcore.toml
// has neither `localDomain` nor `domains` set (e.g. it predates `init`
// seeding them): LOCAL_DOMAIN must fall back to
// "<slugified-project-name>.localhost", and DOMAINS must default to just
// that value, rather than being left blank.
func TestGenerateEnvFile_DomainDefaults(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("projectName", "My Project")

	dir := t.TempDir()
	if err := GenerateEnvFile(dir); err != nil {
		t.Fatalf("GenerateEnvFile error = %v", err)
	}

	localDomain := readEnvVar(t, dir, "LOCAL_DOMAIN")
	if localDomain != "my-project.localhost" {
		t.Errorf("LOCAL_DOMAIN = %q, want %q", localDomain, "my-project.localhost")
	}

	domains := readEnvVar(t, dir, "DOMAINS")
	if domains != "my-project.localhost" {
		t.Errorf("DOMAINS = %q, want %q", domains, "my-project.localhost")
	}
}

// TestGenerateEnvFile_DomainsDefaultsToLocalDomain checks that an explicit
// `localDomain` (but no `domains`) still yields a non-blank DOMAINS,
// defaulting to an array containing just that localDomain.
func TestGenerateEnvFile_DomainsDefaultsToLocalDomain(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("localDomain", "example.localhost")

	dir := t.TempDir()
	if err := GenerateEnvFile(dir); err != nil {
		t.Fatalf("GenerateEnvFile error = %v", err)
	}

	domains := readEnvVar(t, dir, "DOMAINS")
	if domains != "example.localhost" {
		t.Errorf("DOMAINS = %q, want %q", domains, "example.localhost")
	}
}

// TestGenerateEnvFile_DomainsExplicitNotOverridden checks that an explicit
// `domains` list is used as-is, not replaced by the LOCAL_DOMAIN fallback.
func TestGenerateEnvFile_DomainsExplicitNotOverridden(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("localDomain", "example.localhost")
	viper.Set("domains", []string{"example.localhost", "extra.localhost"})

	dir := t.TempDir()
	if err := GenerateEnvFile(dir); err != nil {
		t.Fatalf("GenerateEnvFile error = %v", err)
	}

	domains := readEnvVar(t, dir, "DOMAINS")
	want := "example.localhost extra.localhost"
	if domains != want {
		t.Errorf("DOMAINS = %q, want %q", domains, want)
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
