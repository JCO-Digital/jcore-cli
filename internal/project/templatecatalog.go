package project

import (
	"github.com/JCO-Digital/jcore/container"
	"github.com/pelletier/go-toml/v2"
)

// TemplateCatalogEntry describes one entry in the embedded
// container/templates/templates.toml catalog: which git branch a template
// tracks by default, which branches it offers, the URL of the child theme
// archive to fetch for a new project using it (if any), and any additional
// plugins to install directly from a GitHub release asset rather than via
// Composer or the remote-site plugin sync.
type TemplateCatalogEntry struct {
	Branch       string            `toml:"branch"`
	Branches     []string          `toml:"branches"`
	ThemeURL     string            `toml:"themeUrl"`
	Lohko        bool              `toml:"lohko"`
	Plugins      []string          `toml:"plugins"`
	YdinVersions map[string]string `toml:"ydinVersions"`
}

// YdinVersion returns the ydin constraint configured for branch, falling back
// to the template's default branch if branch is not explicitly mapped.
func (e TemplateCatalogEntry) YdinVersion(branch string) string {
	if branch != "" {
		if v, ok := e.YdinVersions[branch]; ok {
			return v
		}
	}
	if e.Branch != "" {
		if v, ok := e.YdinVersions[e.Branch]; ok {
			return v
		}
	}
	return ""
}

// LoadTemplateCatalog parses the embedded template catalog.
func LoadTemplateCatalog() (map[string]TemplateCatalogEntry, error) {
	raw, err := container.TemplateAssets.ReadFile("templates/templates.toml")
	if err != nil {
		return nil, err
	}

	var catalog map[string]TemplateCatalogEntry
	if err := toml.Unmarshal(raw, &catalog); err != nil {
		return nil, err
	}
	return catalog, nil
}
