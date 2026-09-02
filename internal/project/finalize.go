package project

import (
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

	return renderPhpIni(projectDir, data)
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

	destDir := filepath.Join(projectDir, ".jcore")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(destDir, "php.ini"), rendered, 0644)
}
