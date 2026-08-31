package project

import (
	"os"
	"path/filepath"
)

// FindProjectRoot searches upwards from the current directory for jcore.toml
func FindProjectRoot() (string, error) {
	curr, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for curr != "/" {
		if _, err := os.Stat(filepath.Join(curr, "jcore.toml")); err == nil {
			return curr, nil
		}
		curr = filepath.Dir(curr)
	}

	return "", nil
}

// FindLegacyProjectRoot searches upwards from the current directory for either
// jcore.toml or the legacy config.sh, stopping at the first directory containing
// either one. It reports whether the directory found is a legacy (config.sh-only) project.
func FindLegacyProjectRoot() (dir string, isLegacy bool, err error) {
	curr, err := os.Getwd()
	if err != nil {
		return "", false, err
	}

	for curr != "/" {
		if _, err := os.Stat(filepath.Join(curr, "jcore.toml")); err == nil {
			return curr, false, nil
		}
		if _, err := os.Stat(filepath.Join(curr, LegacyConfigFilename)); err == nil {
			return curr, true, nil
		}
		curr = filepath.Dir(curr)
	}

	return "", false, nil
}
