package project

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	themeNameRe     = regexp.MustCompile(`(?m)^Theme Name:.*$`)
	makefileThemeRe = regexp.MustCompile(`(?m)^theme := .*$`)
	pnpmWorkspaceRe = regexp.MustCompile(`wp-content/themes/[^"'\s]+`)
)

// CreateTheme downloads a child theme archive (substituting "{{branch}}"
// in themeURLTemplate), extracts it into "wp-content/themes/<slug>" under
// projectDir (slug derived from projectName), and rewrites the theme's own
// style.css "Theme Name:" header plus the project's Makefile/
// pnpm-workspace.yaml theme path references to match — mirroring the
// legacy TypeScript CLI's createTheme(). Returns the theme's slug.
func CreateTheme(projectDir, projectName, themeURLTemplate, branch string) (string, error) {
	slug := Slugify(projectName)
	url := strings.ReplaceAll(themeURLTemplate, "{{branch}}", branch)

	extractedDir, err := downloadAndExtractZip(url)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(filepath.Dir(extractedDir))

	themeRelPath := filepath.Join("wp-content", "themes", slug)
	themePath := filepath.Join(projectDir, themeRelPath)
	// The downloaded theme archive is copied verbatim: it's a real,
	// standalone theme repository, not one of jcore's own ".tmpl" scaffold
	// templates.
	if err := copyTemplateTree(os.DirFS(extractedDir), ".", themePath, CurrentTemplateData()); err != nil {
		return "", err
	}

	if err := regexReplaceInFile(filepath.Join(themePath, "style.css"), themeNameRe, "Theme Name: "+projectName); err != nil {
		return "", err
	}
	if err := regexReplaceInFile(filepath.Join(projectDir, "Makefile"), makefileThemeRe, "theme := "+filepath.ToSlash(themeRelPath)); err != nil {
		return "", err
	}
	if err := regexReplaceInFile(filepath.Join(projectDir, "pnpm-workspace.yaml"), pnpmWorkspaceRe, filepath.ToSlash(themeRelPath)); err != nil {
		return "", err
	}

	return slug, nil
}

// regexReplaceInFile replaces every match of re in path's content with
// replacement, in place. A missing file is a no-op, not an error — not
// every project has a Makefile/pnpm-workspace.yaml.
func regexReplaceInFile(path string, re *regexp.Regexp, replacement string) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	updated := re.ReplaceAllLiteralString(string(content), replacement)
	if updated == string(content) {
		return nil
	}
	return os.WriteFile(path, []byte(updated), 0644)
}
