package project

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

const siteConfRelPath = ".config/nginx/site.conf"

// FinalizeProject re-renders the project files whose content depends on
// settings that can change between runs (remote domain, debug mode), such as
// the nginx proxy config and php.ini's xdebug setting.
func FinalizeProject(projectDir string) error {
	data := CurrentTemplateData()

	if err := renderSiteConf(projectDir, data); err != nil {
		return err
	}

	if err := renderPhpIni(projectDir, data); err != nil {
		return err
	}

	return disableAutomaticUpdates(projectDir)
}

// wpConfigRelPath is the generated wp-config.php, bind-mounted into the
// wordpress container's webroot.
const wpConfigRelPath = ".jcore/wordpress/wp-config.php"

// disableAutomaticUpdates defines AUTOMATIC_UPDATER_DISABLED in an existing
// wp-config.php that doesn't define it yet. wp-cron really runs locally
// (the loopback service routes WordPress's requests to its own site), and
// a production database brings its auto_update_plugins setting along, so
// without this WordPress would start updating plugins on its own. Done
// here on the host, before the containers start, since the entrypoint only
// sets it for projects whose docker-entrypoint.sh is up to date.
func disableAutomaticUpdates(projectDir string) error {
	path := filepath.Join(projectDir, wpConfigRelPath)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil // Not installed yet; the entrypoint creates it.
	} else if err != nil {
		return err
	}

	updated, ok := addWPConfigConstant(content, "AUTOMATIC_UPDATER_DISABLED", "true")
	if !ok {
		return nil
	}
	return os.WriteFile(path, updated, 0644)
}

// wpConfigAnchors are the lines wp-config.php constants go right before,
// in order of preference (the first is what wp-cli's `config set` uses).
var wpConfigAnchors = []string{
	"/* That's all, stop editing!",
	"require_once ABSPATH . 'wp-settings.php';",
}

// addWPConfigConstant inserts define(name, rawValue) into wp-config.php
// content. It reports false if name is already mentioned, or there's no
// known anchor to insert it before.
func addWPConfigConstant(content []byte, name, rawValue string) ([]byte, bool) {
	if bytes.Contains(content, []byte("'"+name+"'")) || bytes.Contains(content, []byte(`"`+name+`"`)) {
		return nil, false
	}
	for _, anchor := range wpConfigAnchors {
		i := bytes.Index(content, []byte(anchor))
		if i < 0 {
			continue
		}
		define := fmt.Sprintf("define( '%s', %s );\n", name, rawValue)
		updated := append([]byte{}, content[:i]...)
		updated = append(updated, define...)
		return append(updated, content[i:]...), true
	}
	return nil, false
}

// renderSiteConf re-renders .config/nginx/site.conf in place. The stored
// checksum is only refreshed if the file was unmodified beforehand, so a
// manually customized site.conf keeps being flagged as modified by "jcore update".
func renderSiteConf(projectDir string, data TemplateData) error {
	path := filepath.Join(projectDir, siteConfRelPath)

	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	unmodified, err := CompareChecksum(projectDir, siteConfRelPath, false)
	if err != nil {
		return err
	}

	rendered, err := renderTemplate("site.conf", content, data)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, rendered, 0644); err != nil {
		return err
	}

	if unmodified {
		return UpdateChecksum(projectDir, siteConfRelPath)
	}
	return nil
}

// renderPhpIni renders php.ini's on-disk content (which may have been
// customized) to .jcore/php.ini, the file actually mounted into the container.
// The source php.ini is left untouched so "jcore update" keeps tracking it.
func renderPhpIni(projectDir string, data TemplateData) error {
	content, err := os.ReadFile(filepath.Join(projectDir, "php.ini"))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	rendered, err := renderTemplate("php.ini", content, data)
	if err != nil {
		return err
	}

	// A php.ini scaffolded before xdebugPort existed has no client_port
	// line to render it into; append one (the last value wins).
	if !bytes.Contains(content, []byte("xdebug.client_port")) {
		rendered = append(rendered, fmt.Sprintf("\n[xdebug]\nxdebug.client_port=%d\n", data.XdebugPort)...)
	}

	destDir := filepath.Join(projectDir, ".jcore")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(destDir, "php.ini"), rendered, 0644)
}
