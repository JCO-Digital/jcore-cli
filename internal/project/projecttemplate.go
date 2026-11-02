package project

import (
	"path/filepath"

	"github.com/JCO-Digital/jcore/internal/config"
)

// DefaultTemplate is the template assumed for a project that neither names
// one nor has a recognisable legacy parent-theme submodule.
const DefaultTemplate = "jcore3"

// ProjectTemplate returns the template projectDir was built on. The global
// `template` setting is only the default for scaffolding *new* projects, so
// it is deliberately ignored here: an existing project's template comes from
// its own config files (local, jcore.toml, defaults.toml). Projects scaffolded by older CLIs
// often have neither key, so for those it's detected from the parent theme
// committed as a submodule (jcore2/jcore1), falling back to DefaultTemplate.
// Without this, `jcore update project` on such a jcore2 project rendered the
// jcore3 template over it, including its defaults.toml.
func ProjectTemplate(projectDir string) string {
	// Same precedence as the config loader: local over jcore.toml over
	// defaults.toml, each with its branch overlay applied.
	paths := []string{}
	if p, err := config.GetConfigPath(config.ScopeLocal, projectDir); err == nil {
		paths = append(paths, p)
	}
	if p, err := config.GetConfigPath(config.ScopeProject, projectDir); err == nil {
		paths = append(paths, p)
	}
	paths = append(paths, filepath.Join(projectDir, "defaults.toml"))

	branch := CurrentBranch(projectDir)
	for _, path := range paths {
		values, err := config.LoadTOMLWithBranchOverlay(path, branch)
		if err != nil {
			continue
		}
		if t, ok := values["template"].(string); ok && t != "" {
			return t
		}
	}
	if t := DetectLegacyTemplate(projectDir); t != "blank" {
		return t
	}
	return DefaultTemplate
}
