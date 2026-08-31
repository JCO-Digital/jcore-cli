package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/JCO-Digital/jcore/container"
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

// UpdateProject updates the project files from the embedded assets.
// If only is non-empty, exclusively those files are considered (and always
// overwritten, regardless of checksum). Otherwise every file is considered
// using the normal checksum-based rules, except files listed in force, which
// are always overwritten regardless of checksum status.
func UpdateProject(projectDir string, only []string, force []string) error {
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
	err = updateFromFS(container.BaseAssets, "base", projectDir, checksums, only, force, data)
	if err != nil {
		return fmt.Errorf("failed to update base assets: %w", err)
	}

	// 2. Update Template Assets
	templatePath := filepath.Join("templates", template)
	err = updateFromFS(container.TemplateAssets, templatePath, projectDir, checksums, only, force, data)
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

// DetectModifiedFiles returns the relative paths of scaffolded files that
// have a stored checksum but no longer match it - i.e. files that "jcore
// update" would normally skip because they appear to have been modified.
func DetectModifiedFiles(projectDir string) ([]string, error) {
	checksums, err := LoadChecksums(projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load checksums: %w", err)
	}

	template := viper.GetString("template")
	if template == "" {
		template = "jcore3"
	}

	modified := make(map[string]bool)
	collect := func(srcFS fs.FS, srcRoot string) {
		_ = fs.WalkDir(srcFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			relPath, _ := filepath.Rel(srcRoot, path)
			destRelPath := strings.TrimSuffix(relPath, TemplateExt)
			if destRelPath == "jcore.toml" || destRelPath == ".localConfig.toml" {
				return nil
			}

			storedChecksum, exists := checksums[destRelPath]
			if !exists {
				return nil
			}

			currentChecksum, err := CalculateChecksum(filepath.Join(projectDir, destRelPath))
			if err != nil {
				// Missing or unreadable: not a "modified" case.
				return nil
			}

			if currentChecksum != storedChecksum {
				modified[destRelPath] = true
			}

			return nil
		})
	}

	collect(container.BaseAssets, "base")
	collect(container.TemplateAssets, filepath.Join("templates", template))

	result := make([]string, 0, len(modified))
	for f := range modified {
		result = append(result, f)
	}
	sort.Strings(result)

	return result, nil
}

func updateFromFS(srcFS fs.FS, srcRoot, destRoot string, checksums map[string]string, only []string, force []string, data TemplateData) error {
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

		// If "only" is set, exclusively consider those files.
		if len(only) > 0 && !slices.Contains(only, destRelPath) {
			return nil
		}

		// Forced files are always overwritten, regardless of checksum status.
		forced := slices.Contains(force, destRelPath)

		shouldUpdate := forced
		if !forced {
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				// File doesn't exist, always create it
				shouldUpdate = true
			} else {
				// File exists, check checksum
				storedChecksum, exists := checksums[destRelPath]
				if !exists {
					// No stored checksum, safer to skip unless forced (but we'll stick to original logic for now)
					// Or maybe it's a new file in the base assets that wasn't there before
					fmt.Printf("  [ SKIP ] %s (no checksum found, manual modification possible)\n", destRelPath)
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
					fmt.Printf("  [ SKIP ] %s (modified by user)\n", destRelPath)
					return nil
				}
			}
		}

		if shouldUpdate {
			status := "  OK  "
			if forced {
				status = "FORCED"
			}
			fmt.Printf("  [%s] Updating %s\n", status, destRelPath)
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
