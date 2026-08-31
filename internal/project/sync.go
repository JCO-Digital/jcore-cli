package project

import (
	"github.com/JCO-Digital/jcore/internal/docker"
)

// SyncPlugins runs the container's importplugins script, which rsyncs plugins
// from the configured remote host (skipping PLUGIN_EXCLUDE/PLUGIN_GIT ones)
// unless pluginInstall is set to "composer" or "local".
func SyncPlugins(projectDir string) error {
	return docker.ComposeExec(projectDir, "wordpress", []string{"/project/.config/scripts/importplugins"})
}

// SyncMedia runs the container's importmedia script, which rsyncs uploads
// from the configured remote host.
func SyncMedia(projectDir string) error {
	return docker.ComposeExec(projectDir, "wordpress", []string{"/project/.config/scripts/importmedia"})
}

// InstallLocalPlugins runs the container's installplugins script, which
// installs and activates every plugin listed in pluginLocal.
func InstallLocalPlugins(projectDir string) error {
	return docker.ComposeExec(projectDir, "wordpress", []string{"/project/.config/scripts/installplugins"})
}
