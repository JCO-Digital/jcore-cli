package project

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/JCO-Digital/jcore/container"
)

const lohkoTemplatesRoot = "lohko-templates"

// LohkoBlockData holds the values available to a Lohko block's ".tmpl"
// template files (see container/lohko-templates).
type LohkoBlockData struct {
	Name string
	Slug string
}

// LohkoBlockTemplateNames returns the available Lohko block template names
// (e.g. "dynamic", "static"), embedded under container/lohko-templates.
func LohkoBlockTemplateNames() ([]string, error) {
	entries, err := container.LohkoTemplates.ReadDir(lohkoTemplatesRoot)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// CreateBlock creates a new Lohko Gutenberg block at destDir from the named
// embedded template ("dynamic"/"static"), mirroring the legacy TypeScript
// CLI's createBlock(): copies every template file (rendering each ".tmpl"
// one against LohkoBlockData, dropping the suffix), then rewrites
// block.json's title/name/description to match.
func CreateBlock(destDir, template, name, slug, description string) error {
	sourceDir := filepath.Join(lohkoTemplatesRoot, template)
	data := LohkoBlockData{Name: name, Slug: slug}

	if err := copyTemplateTree(container.LohkoTemplates, sourceDir, destDir, data); err != nil {
		return err
	}

	return setBlockMetadata(filepath.Join(destDir, "block.json"), name, slug, description)
}

// setBlockMetadata rewrites a copied block.json's title/name/description to
// match the block just created, exactly like the legacy CLI's createBlock().
func setBlockMetadata(path, name, slug, description string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	data["title"] = name
	// "lohko/", not "jcore/": WordPress core strips a "core/" prefix when
	// generating a block's default CSS class, but does the same blind
	// substring substitution for every block namespace — "jcore/foo" would
	// collide with that and render as "wp-block-jfoo" instead of
	// "wp-block-jcore-foo".
	data["name"] = "lohko/" + slug
	data["description"] = description

	encoded, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0644)
}
