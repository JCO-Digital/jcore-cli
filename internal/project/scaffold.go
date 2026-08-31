package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/JCO-Digital/jcore/internal/container"
	"github.com/spf13/viper"
)

// ScaffoldProject extracts the embedded base and template assets to the target directory
func ScaffoldProject(targetDir string, template string) error {
	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	data := CurrentTemplateData()
	checksums := make(map[string]string)

	// 1. Scaffold Base Assets
	err := fs.WalkDir(container.BaseAssets, "base", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel("base", path)

		if d.IsDir() {
			if relPath == "." {
				return nil
			}
			return os.MkdirAll(filepath.Join(targetDir, relPath), 0755)
		}

		destRelPath := strings.TrimSuffix(relPath, TemplateExt)
		destPath := filepath.Join(targetDir, destRelPath)

		if err := copyOrRenderFile(container.BaseAssets, path, destPath, data); err != nil {
			return err
		}

		checksum, _ := CalculateChecksum(destPath)
		checksums[destRelPath] = checksum

		if filepath.HasPrefix(destRelPath, ".config/scripts") || filepath.Ext(destRelPath) == ".sh" {
			_ = os.Chmod(destPath, 0755)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// 2. Scaffold Template Assets
	if template != "" {
		templateRoot := filepath.Join("templates", template)
		_ = fs.WalkDir(container.TemplateAssets, templateRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // Skip if template directory doesn't exist
			}

			relPath, _ := filepath.Rel(templateRoot, path)
			if relPath == "." {
				return nil
			}

			if d.IsDir() {
				return os.MkdirAll(filepath.Join(targetDir, relPath), 0755)
			}

			destRelPath := strings.TrimSuffix(relPath, TemplateExt)
			destPath := filepath.Join(targetDir, destRelPath)

			if err := copyOrRenderFile(container.TemplateAssets, path, destPath, data); err != nil {
				return err
			}

			checksum, _ := CalculateChecksum(destPath)
			checksums[destRelPath] = checksum

			return nil
		})
	}

	return SaveChecksums(targetDir, checksums)
}

// copyOrRenderFile copies srcPath to destPath. If srcPath ends in TemplateExt,
// its content is executed as a Go template against data first.
func copyOrRenderFile(srcFS fs.FS, srcPath, destPath string, data TemplateData) error {
	content, err := fs.ReadFile(srcFS, srcPath)
	if err != nil {
		return err
	}

	if strings.HasSuffix(srcPath, TemplateExt) {
		content, err = renderTemplate(filepath.Base(srcPath), content, data)
		if err != nil {
			return fmt.Errorf("failed to render template %s: %w", srcPath, err)
		}
	}

	return os.WriteFile(destPath, content, 0644)
}

// UpdateProject updates the project files from the embedded assets
func UpdateProject(projectDir string, include []string) error {
	checksums, err := LoadChecksums(projectDir)
	if err != nil {
		return fmt.Errorf("failed to load checksums: %w", err)
	}

	template := viper.GetString("template")
	if template == "" {
		template = "jcore3"
	}

	fmt.Printf("Updating project using template: %s\n", template)

	data := CurrentTemplateData()

	// 1. Update Base Assets
	err = updateFromFS(container.BaseAssets, "base", projectDir, checksums, include, data)
	if err != nil {
		return fmt.Errorf("failed to update base assets: %w", err)
	}

	// 2. Update Template Assets
	templatePath := filepath.Join("templates", template)
	err = updateFromFS(container.TemplateAssets, templatePath, projectDir, checksums, include, data)
	if err != nil {
		// Template might not exist in embedded assets, just warn if it's not "jcore3"
		if template != "jcore3" {
			fmt.Printf("Warning: Template assets for '%s' not found or failed to update.\n", template)
		}
	}

	// Save updated checksums
	if err := SaveChecksums(projectDir, checksums); err != nil {
		return fmt.Errorf("failed to save checksums: %w", err)
	}

	return nil
}

func updateFromFS(srcFS fs.FS, srcRoot, destRoot string, checksums map[string]string, include []string, data TemplateData) error {
	return fs.WalkDir(srcFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(srcRoot, path)
		if relPath == "." {
			return nil
		}

		destRelPath := strings.TrimSuffix(relPath, TemplateExt)

		// Skip some files that should never be overwritten during update
		if destRelPath == "jcore.toml" || destRelPath == ".localConfig.toml" {
			return nil
		}

		destPath := filepath.Join(destRoot, destRelPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// If specific files are targeted, only consider those - and always
		// overwrite a targeted file, regardless of checksum status.
		targeted := false
		if len(include) > 0 {
			for _, inc := range include {
				if inc == destRelPath {
					targeted = true
					break
				}
			}
			if !targeted {
				return nil
			}
		}

		shouldUpdate := targeted
		if !targeted {
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				// File doesn't exist, always create it
				shouldUpdate = true
			} else {
				// File exists, check checksum
				storedChecksum, exists := checksums[destRelPath]
				if !exists {
					// No stored checksum, safer to skip unless forced (but we'll stick to original logic for now)
					// Or maybe it's a new file in the base assets that wasn't there before
					fmt.Printf("  [SKIP] %s (no checksum found, manual modification possible)\n", destRelPath)
					return nil
				}

				currentChecksum, err := CalculateChecksum(destPath)
				if err != nil {
					return err
				}

				if currentChecksum == storedChecksum {
					// File hasn't been modified by user, safe to update
					shouldUpdate = true
				} else {
					fmt.Printf("  [SKIP] %s (modified by user)\n", destRelPath)
					return nil
				}
			}
		}

		if shouldUpdate {
			fmt.Printf("  [ OK ] Updating %s\n", destRelPath)
			if err := copyOrRenderFile(srcFS, path, destPath, data); err != nil {
				return err
			}

			// Update checksum
			newChecksum, _ := CalculateChecksum(destPath)
			checksums[destRelPath] = newChecksum

			// Set permissions if needed
			if filepath.HasPrefix(destRelPath, ".config/scripts") || filepath.Ext(destRelPath) == ".sh" {
				_ = os.Chmod(destPath, 0755)
			}
		}

		return nil
	})
}
