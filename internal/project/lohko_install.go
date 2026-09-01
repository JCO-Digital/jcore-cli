package project

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LohkoPluginRelPath is the Lohko plugin's location within a project.
const LohkoPluginRelPath = "wp-content/plugins/lohko"

// LohkoPluginURL is the source archive InstallLohko downloads — always
// fetched fresh from GitHub, since (unlike the block templates mirrored
// under container/lohko-templates) the plugin itself is a live, actively
// maintained codebase, much like the jcore-ilme theme (see theme.go).
const LohkoPluginURL = "https://github.com/JCO-Digital/jcore-lohko/archive/refs/heads/main.zip"

// LohkoInstalled reports whether the Lohko plugin is present in projectDir.
func LohkoInstalled(projectDir string) bool {
	_, err := os.Stat(filepath.Join(projectDir, LohkoPluginRelPath))
	return err == nil
}

// InstallLohko downloads and extracts the Lohko plugin into
// "wp-content/plugins/lohko" under projectDir.
func InstallLohko(projectDir string) error {
	extractedDir, err := downloadAndExtractZip(LohkoPluginURL)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(extractedDir))

	dest := filepath.Join(projectDir, LohkoPluginRelPath)
	return copyTemplateTree(os.DirFS(extractedDir), ".", dest, CurrentTemplateData())
}

// LohkoBundledBlock is one example block bundled with a freshly downloaded
// Lohko plugin, discovered by LohkoBundledBlocks.
type LohkoBundledBlock struct {
	Folder      string
	Title       string
	Description string
}

// LohkoBundledBlocks lists the example blocks bundled under a project's
// installed Lohko plugin (each a "src" subfolder with a block.json
// declaring a "title"), so the caller can offer to prune the ones it
// doesn't want.
func LohkoBundledBlocks(projectDir string) ([]LohkoBundledBlock, error) {
	srcDir := filepath.Join(projectDir, LohkoPluginRelPath, "src")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}

	var blocks []LohkoBundledBlock
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(srcDir, e.Name(), "block.json"))
		if err != nil {
			continue
		}

		var data struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(raw, &data); err != nil || data.Title == "" {
			continue
		}

		blocks = append(blocks, LohkoBundledBlock{Folder: e.Name(), Title: data.Title, Description: data.Description})
	}
	return blocks, nil
}

// RemoveLohkoBlock deletes one bundled example block's folder.
func RemoveLohkoBlock(projectDir, folder string) error {
	return os.RemoveAll(filepath.Join(projectDir, LohkoPluginRelPath, "src", folder))
}
