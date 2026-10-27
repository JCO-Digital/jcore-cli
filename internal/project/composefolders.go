package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// composeFile is the minimal shape of a compose file this package needs:
// enough to find every service's bind-mounted host folders.
type composeFile struct {
	Services map[string]struct {
		Volumes []any `yaml:"volumes"`
	} `yaml:"services"`
}

// ComposeMountedFolders parses the compose files projectDir runs on (see
// ResolveComposeFiles) and returns every project-relative bind-mount
// source path (e.g. "./wp-content") they declare across all services,
// deduplicated and sorted. A missing or unparsable file just yields no
// folders, not an error.
func ComposeMountedFolders(projectDir string) ([]string, error) {
	setup, err := ResolveComposeFiles(projectDir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var folders []string
	for _, file := range setup.Files {
		raw, err := os.ReadFile(filepath.Join(projectDir, file))
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		var compose composeFile
		if err := yaml.Unmarshal(raw, &compose); err != nil {
			continue
		}

		for _, service := range compose.Services {
			for _, v := range service.Volumes {
				entry, ok := v.(string)
				if !ok {
					continue // long (map) mount syntax isn't handled
				}
				source, ok := bindMountSource(entry)
				if !ok || seen[source] {
					continue
				}
				seen[source] = true
				folders = append(folders, source)
			}
		}
	}

	sort.Strings(folders)
	return folders, nil
}

// bindMountSource extracts the host-side path from a short-syntax compose
// volume entry ("<source>:<target>[:<mode>]"), if it's a project-relative
// bind mount worth pre-creating. Requiring the "./" prefix deliberately
// excludes: named volumes ("db:/var/lib/mysql"), absolute or
// home-relative paths ("/x", "~/x" — outside the project; global folders
// like "~/.config/jcore/ssl" are already handled by doctor.go's own
// checks), anything escaping the project root ("../x"), variable/shell
// expansions ("${VAR:-/dev/null}"), and the project root itself
// (".:/project", which obviously already exists).
func bindMountSource(entry string) (string, bool) {
	entry = strings.TrimSpace(entry)
	if !strings.HasPrefix(entry, "./") {
		return "", false
	}

	idx := strings.IndexByte(entry, ':')
	if idx <= 0 {
		return "", false
	}
	return entry[:idx], true
}

// EnsureComposeMountedFolders creates every ComposeMountedFolders entry
// under projectDir that doesn't already exist, as a plain directory owned
// by the current user — so Docker never has to create the bind-mount
// point itself (as root) when `docker compose up` runs, which otherwise
// leaves it in a state build scripts running as a normal user can't write
// into. A path that already exists — whether a directory, or a file such
// as ".jcore/php.ini" (rendered by FinalizeProject before this runs) — is
// left untouched.
func EnsureComposeMountedFolders(projectDir string) error {
	folders, err := ComposeMountedFolders(projectDir)
	if err != nil {
		return err
	}

	for _, folder := range folders {
		path := filepath.Join(projectDir, folder)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", path, err)
		}
	}
	return nil
}
