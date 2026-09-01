package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// copyTemplateTree copies every file under srcRoot in srcFS to destRoot on
// disk, skipping ".git" directories — mirroring ScaffoldProject's own
// embedded-asset copying. Any file ending in TemplateExt (".tmpl") is
// executed as a Go template against data first, with the suffix dropped
// from the destination filename; everything else is copied verbatim.
func copyTemplateTree(srcFS fs.FS, srcRoot, destRoot string, data any) error {
	if err := os.MkdirAll(destRoot, 0755); err != nil {
		return err
	}

	return fs.WalkDir(srcFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}

		relPath, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(destRoot, strings.TrimSuffix(relPath, TemplateExt))

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		return copyOrRenderFile(srcFS, path, destPath, data)
	})
}
