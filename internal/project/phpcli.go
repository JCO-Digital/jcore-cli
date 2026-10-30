package project

import (
	"bytes"
	"os"
	"path/filepath"
)

// Quiet PHP settings for `jcore run`. They're written into the project
// and loaded only by the commands jcore run starts, through
// PHP_INI_SCAN_DIR, so the site served by php-fpm keeps the project's own
// php.ini. The project root is mounted at /project in the wordpress
// container.
const (
	quietPHPRelDir       = ".jcore/php-cli"
	quietPHPContainerDir = "/project/" + quietPHPRelDir
	quietPHPIni          = "jcore-run.ini"
	quietPHPPrepend      = "jcore-run-prepend.php"
)

const quietPHPIniContent = `; Written by jcore run and loaded only by the commands it runs (through
; PHP_INI_SCAN_DIR), never by the site. Rewritten on every run; pass
; --notices to jcore run to go without it.

; Hide deprecations and notices, e.g. the ones wp-cli itself triggers while
; it boots on a newer PHP.
error_reporting = E_ALL & ~E_DEPRECATED & ~E_USER_DEPRECATED & ~E_NOTICE & ~E_USER_NOTICE

; Report the remaining errors only through the error log, which is stderr
; on the CLI, so they never end up in the command's stdout.
display_errors = Off
log_errors = On

; wp-cli raises error_reporting back to E_ALL once WordPress loads with
; WP_DEBUG on, so drop deprecations and notices with an error handler too.
auto_prepend_file = ` + quietPHPContainerDir + "/" + quietPHPPrepend + `
`

const quietPHPPrependContent = `<?php
// Written by jcore run, see jcore-run.ini. Drops deprecations and notices
// however high WordPress or wp-cli set error_reporting later.
set_error_handler(
	static function () {
		return true;
	},
	E_DEPRECATED | E_USER_DEPRECATED | E_NOTICE | E_USER_NOTICE
);
`

// WriteQuietPHPConfig writes jcore run's quiet PHP settings into
// projectDir, leaving files that are already up to date untouched, and
// returns the NAME=value environment entry that makes PHP load them
// alongside its usual configuration.
func WriteQuietPHPConfig(projectDir string) (string, error) {
	dir := filepath.Join(projectDir, quietPHPRelDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	files := map[string]string{
		quietPHPIni:     quietPHPIniContent,
		quietPHPPrepend: quietPHPPrependContent,
	}
	for name, content := range files {
		if err := writeIfChanged(filepath.Join(dir, name), []byte(content)); err != nil {
			return "", err
		}
	}
	// A leading ":" keeps PHP's compiled-in scan dir (conf.d, which loads
	// the extensions) and adds this one after it.
	return "PHP_INI_SCAN_DIR=:" + quietPHPContainerDir, nil
}

// writeIfChanged writes content to path unless it already holds exactly
// that. It writes a temporary file and renames it into place, so a command
// starting concurrently never reads a half-written file.
func writeIfChanged(path string, content []byte) error {
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, content) {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
