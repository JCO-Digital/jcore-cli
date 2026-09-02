package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func setupFinalizeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	siteConfDir := filepath.Join(dir, ".config", "nginx")
	if err := os.MkdirAll(siteConfDir, 0755); err != nil {
		t.Fatalf("failed to create fixture dirs: %v", err)
	}

	siteConf := "location @upstream {\n    proxy_pass https://{{.RemoteDomain}};\n}\n"
	if err := os.WriteFile(filepath.Join(siteConfDir, "site.conf"), []byte(siteConf), 0644); err != nil {
		t.Fatalf("failed to write site.conf fixture: %v", err)
	}

	phpIni := "[xdebug]\nxdebug.mode={{.XdebugMode}}\n"
	if err := os.WriteFile(filepath.Join(dir, "php.ini"), []byte(phpIni), 0644); err != nil {
		t.Fatalf("failed to write php.ini fixture: %v", err)
	}

	checksums := map[string]string{}
	siteConfChecksum, err := CalculateChecksum(filepath.Join(siteConfDir, "site.conf"))
	if err != nil {
		t.Fatalf("failed to checksum site.conf fixture: %v", err)
	}
	checksums[siteConfRelPath] = siteConfChecksum
	if err := SaveChecksums(dir, checksums); err != nil {
		t.Fatalf("failed to save checksums: %v", err)
	}

	return dir
}

func TestFinalizeProjectRendersAndRefreshesChecksum(t *testing.T) {
	dir := setupFinalizeFixture(t)

	viper.Reset()
	viper.Set("remoteDomain", "example.org")
	viper.Set("debug", true)
	t.Cleanup(viper.Reset)

	if err := FinalizeProject(dir); err != nil {
		t.Fatalf("FinalizeProject failed: %v", err)
	}

	siteConf, err := os.ReadFile(filepath.Join(dir, siteConfRelPath))
	if err != nil {
		t.Fatalf("failed to read rendered site.conf: %v", err)
	}
	if !strings.Contains(string(siteConf), "proxy_pass https://example.org;") {
		t.Errorf("site.conf not rendered correctly: %s", siteConf)
	}

	phpIni, err := os.ReadFile(filepath.Join(dir, ".jcore", "php.ini"))
	if err != nil {
		t.Fatalf("failed to read rendered .jcore/php.ini: %v", err)
	}
	if !strings.Contains(string(phpIni), "xdebug.mode=develop,debug") {
		t.Errorf(".jcore/php.ini not rendered correctly: %s", phpIni)
	}

	// The source php.ini must be left untouched.
	sourcePhpIni, err := os.ReadFile(filepath.Join(dir, "php.ini"))
	if err != nil {
		t.Fatalf("failed to read source php.ini: %v", err)
	}
	if !strings.Contains(string(sourcePhpIni), "{{.XdebugMode}}") {
		t.Errorf("source php.ini should remain unrendered, got: %s", sourcePhpIni)
	}

	// Checksum should have been refreshed to match the freshly rendered site.conf,
	// so a second finalize run with the same settings sees it as unmodified.
	match, err := CompareChecksum(dir, siteConfRelPath, false)
	if err != nil {
		t.Fatalf("CompareChecksum failed: %v", err)
	}
	if !match {
		t.Errorf("expected checksum to be refreshed after rendering, but it didn't match")
	}
}

func TestFinalizeProjectPreservesUserModifiedSiteConf(t *testing.T) {
	dir := setupFinalizeFixture(t)

	// Simulate the user hand-editing site.conf after it was scaffolded, without
	// updating the stored checksum.
	custom := "location @upstream {\n    proxy_pass https://{{.RemoteDomain}}; # custom edit\n}\n"
	if err := os.WriteFile(filepath.Join(dir, siteConfRelPath), []byte(custom), 0644); err != nil {
		t.Fatalf("failed to write custom site.conf: %v", err)
	}

	viper.Reset()
	viper.Set("remoteDomain", "example.org")
	t.Cleanup(viper.Reset)

	if err := FinalizeProject(dir); err != nil {
		t.Fatalf("FinalizeProject failed: %v", err)
	}

	// The checksum should NOT have been refreshed, so the file still shows as
	// modified for "jcore update" purposes.
	match, err := CompareChecksum(dir, siteConfRelPath, false)
	if err != nil {
		t.Fatalf("CompareChecksum failed: %v", err)
	}
	if match {
		t.Errorf("expected checksum to remain stale for a user-modified file")
	}
}
