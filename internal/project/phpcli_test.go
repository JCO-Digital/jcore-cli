package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteQuietPHPConfig(t *testing.T) {
	projectDir := t.TempDir()

	env, err := WriteQuietPHPConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if env != "PHP_INI_SCAN_DIR=:/project/.jcore/php-cli" {
		t.Errorf("env = %q", env)
	}

	dir := filepath.Join(projectDir, ".jcore", "php-cli")
	ini, err := os.ReadFile(filepath.Join(dir, "jcore-run.ini"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"error_reporting = E_ALL & ~E_DEPRECATED & ~E_USER_DEPRECATED & ~E_NOTICE & ~E_USER_NOTICE\n",
		"display_errors = Off\n",
		"auto_prepend_file = /project/.jcore/php-cli/jcore-run-prepend.php\n",
	} {
		if !strings.Contains(string(ini), want) {
			t.Errorf("jcore-run.ini lacks %q:\n%s", want, ini)
		}
	}
	prepend, err := os.ReadFile(filepath.Join(dir, "jcore-run-prepend.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(prepend), "<?php\n") || !strings.Contains(string(prepend), "set_error_handler(") {
		t.Errorf("unexpected jcore-run-prepend.php:\n%s", prepend)
	}

	// Up-to-date files are left alone; stale ones are rewritten.
	iniPath := filepath.Join(dir, "jcore-run.ini")
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(iniPath, old, old); err != nil {
		t.Fatal(err)
	}
	prependPath := filepath.Join(dir, "jcore-run-prepend.php")
	if err := os.WriteFile(prependPath, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteQuietPHPConfig(projectDir); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(iniPath); err != nil || !info.ModTime().Equal(old) {
		t.Errorf("unchanged jcore-run.ini was rewritten")
	}
	if got, _ := os.ReadFile(prependPath); string(got) != string(prepend) {
		t.Errorf("stale jcore-run-prepend.php was not rewritten: %q", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Errorf("expected only the two config files, found %d entries", len(entries))
	}
}
