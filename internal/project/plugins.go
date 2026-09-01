package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstallGithubPlugin downloads a plugin from a GitHub release asset URL
// (typically "https://github.com/<owner>/<repo>/releases/latest/download/
// <name>.zip", which always resolves to that repo's current latest release)
// into "wp-content/plugins/<name>" under projectDir, where <name> is the
// asset's filename without its ".zip" suffix.
//
// Unlike a GitHub source-code archive (see CreateTheme/downloadAndExtractZip),
// a release asset built for this purpose has no wrapping "<repo>-<branch>/"
// folder — its files already sit at the zip root, ready to extract directly
// into the plugin's own folder — so this doesn't need that unwrapping step.
func InstallGithubPlugin(projectDir, url string) (string, error) {
	name := strings.TrimSuffix(filepath.Base(url), ".zip")
	if name == "" || name == "." || name == "/" {
		return "", fmt.Errorf("cannot determine a plugin name from url %q", url)
	}

	data, err := downloadZip(url)
	if err != nil {
		return "", err
	}

	dest := filepath.Join(projectDir, "wp-content", "plugins", name)
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", err
	}
	if err := extractZip(data, dest); err != nil {
		return "", err
	}

	return name, nil
}

// MergeUnique appends any of add not already present in base, preserving
// base's original order and deduping add against itself too.
func MergeUnique(base, add []string) []string {
	seen := make(map[string]bool, len(base))
	merged := make([]string, len(base))
	copy(merged, base)
	for _, v := range base {
		seen[v] = true
	}
	for _, v := range add {
		if !seen[v] {
			seen[v] = true
			merged = append(merged, v)
		}
	}
	return merged
}
