package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Scope int

const (
	ScopeGlobal Scope = iota
	ScopeProject
	ScopeLocal
)

// String renders a scope for display (e.g. in `config list` or the TUI).
func (s Scope) String() string {
	switch s {
	case ScopeGlobal:
		return "Global"
	case ScopeProject:
		return "Project"
	case ScopeLocal:
		return "Local"
	default:
		return "Unknown"
	}
}

const (
	GlobalConfigName  = "config.toml"
	ProjectConfigName = "jcore.toml"
	LocalConfigName   = ".localConfig.toml"
)

// GetConfigPath resolves the on-disk path for scope. For ScopeProject and
// ScopeLocal, projectRoot must be a non-empty directory (typically from
// project.FindProjectRoot()) — these scopes have no meaning relative to the
// current working directory, only relative to the project root.
func GetConfigPath(scope Scope, projectRoot string) (string, error) {
	switch scope {
	case ScopeGlobal:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "jcore", GlobalConfigName), nil
	case ScopeProject:
		if projectRoot == "" {
			return "", fmt.Errorf("not in a jcore project")
		}
		return filepath.Join(projectRoot, ProjectConfigName), nil
	case ScopeLocal:
		if projectRoot == "" {
			return "", fmt.Errorf("not in a jcore project")
		}
		return filepath.Join(projectRoot, LocalConfigName), nil
	default:
		return "", fmt.Errorf("invalid scope")
	}
}
