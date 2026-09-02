package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Value/key colors mirror the legacy TypeScript CLI's chalk-based
// formatValue/listConfig (key green, string cyan, number yellow, boolean
// magenta, list items blue). Scope colors mirror the TUI's own scope
// badges (internal/tui/styles.go) for visual consistency across both.
var (
	styleHeading = lipgloss.NewStyle().Bold(true)
	styleKey     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleString  = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleNumber  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleBool    = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	styleList    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	styleScopeDefault = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleScopeGlobal  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleScopeProject = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleScopeLocal   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

// colorizeValue renders v for `config list` display, colored by its Go
// type the same way the legacy CLI colored settings by their schema type.
// v may come already normalized (int, []string — from config.Store/
// config.ProjectDefaultsFile) or still in go-toml's raw decoded form
// (int64, []any — from viper.AllSettings(), which the real merge pipeline
// in cmd/root.go feeds directly from parsed TOML with no normalization
// pass), so both are handled.
func colorizeValue(v any) string {
	switch val := v.(type) {
	case bool:
		return styleBool.Render(fmt.Sprintf("%v", val))
	case int, int64:
		return styleNumber.Render(fmt.Sprintf("%v", val))
	case []string, []any:
		return styleList.Render(fmt.Sprintf("%v", val))
	case string:
		return styleString.Render(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// colorizeScopeLabel wraps a scope label (e.g. "project", "project@staging")
// in the style matching its scope word (the part before "@", if any).
func colorizeScopeLabel(label string) string {
	word := label
	if i := strings.IndexByte(label, '@'); i >= 0 {
		word = label[:i]
	}

	switch word {
	case "global":
		return styleScopeGlobal.Render(label)
	case "project":
		return styleScopeProject.Render(label)
	case "local":
		return styleScopeLocal.Render(label)
	default:
		return styleScopeDefault.Render(label)
	}
}
