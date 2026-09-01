package project

import (
	"github.com/JCO-Digital/jcore/container"
	"github.com/pelletier/go-toml/v2"
)

// TemplateCatalogEntry describes one entry in the embedded
// container/templates/templates.toml catalog: which git branch a template
// tracks by default, which branches it offers, and (if any) the URL of the
// child theme archive to fetch for a new project using it.
type TemplateCatalogEntry struct {
	Branch   string   `toml:"branch"`
	Branches []string `toml:"branches"`
	ThemeURL string   `toml:"themeUrl"`
	Lohko    bool     `toml:"lohko"`
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
