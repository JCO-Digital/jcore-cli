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

const (
	GlobalConfigName  = "config.toml"
	ProjectConfigName = "jcore.toml"
	LocalConfigName   = ".localConfig.toml"
)

func GetConfigPath(scope Scope) (string, error) {
	switch scope {
	case ScopeGlobal:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "jcore", GlobalConfigName), nil
	case ScopeProject:
		return ProjectConfigName, nil
	case ScopeLocal:
		return LocalConfigName, nil
	default:
		return "", fmt.Errorf("invalid scope")
	}
}
