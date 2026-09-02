package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/JCO-Digital/jcore/internal/config"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// settingItem is one editable row: a known setting plus its currently
// resolved effective value/source scope.
type settingItem struct {
	def        config.SettingDef
	resolution config.Resolution
}

func (i settingItem) FilterValue() string {
	return i.def.Key + " " + i.def.Description
}

// headerItem is a non-editable category separator row. FilterValue returns
// "" so it never matches a filter query and is hidden while filtering.
type headerItem struct {
	category string
}

func (h headerItem) FilterValue() string { return "" }

// buildItems constructs the full, ordered list of rows (category headers
// interleaved with their settings), each setting's value freshly resolved
// against projectRoot/branch.
func buildItems(projectRoot, branch string) []list.Item {
	var items []list.Item
	for _, category := range config.Categories() {
		items = append(items, headerItem{category: category})
		for _, def := range config.InCategory(category) {
			res, err := config.Resolve(def.Key, projectRoot, branch)
			if err != nil {
				res = config.Resolution{IsDefault: true}
			}
			items = append(items, settingItem{def: def, resolution: res})
		}
	}
	return items
}

// rowDelegate renders both settingItem and headerItem rows. It carries the
// active branch name (if any) purely for display, so the scope badge can
// show e.g. "[project@staging]" when a value comes from that branch's
// override table rather than the file's top-level settings.
type rowDelegate struct {
	branch string
}

func (d rowDelegate) Height() int  { return 1 }
func (d rowDelegate) Spacing() int { return 0 }

func (d rowDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d rowDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	switch it := item.(type) {
	case headerItem:
		fmt.Fprint(w, styleHeader.Render(it.category))
	case settingItem:
		value := formatValue(it.def, it.resolution.Value)
		badge := scopeBadge(it.resolution, d.branch)
		line := fmt.Sprintf("%-20s %-32s %s", it.def.Key, value, badge)
		if index == m.Index() {
			fmt.Fprint(w, styleSelected.Render("> "+line))
		} else {
			fmt.Fprint(w, "  "+line)
		}
	}
}

// formatValue renders a setting's value for the list row, masking
// sensitive values.
func formatValue(def config.SettingDef, value any) string {
	if def.Sensitive {
		if value == nil || fmt.Sprintf("%v", value) == "" {
			return ""
		}
		return "••••••"
	}
	return formatForEdit(def, value)
}

// formatForEdit renders a setting's value as the string a text input
// should be pre-filled with (unmasked, even for Sensitive settings).
func formatForEdit(def config.SettingDef, value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case []string:
		return strings.Join(v, ", ")
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// scopeBadge renders a short tag showing which scope a value's effective
// value actually comes from, and — if it came from a branch override table
// — which branch, e.g. "[project@staging]".
func scopeBadge(res config.Resolution, branch string) string {
	if res.IsDefault {
		return styleBadgeDefault.Render("[default]")
	}

	label := strings.ToLower(res.SourceScope.String())
	if res.FromBranchOverride && branch != "" {
		label += "@" + branch
	}
	text := "[" + label + "]"

	switch res.SourceScope {
	case config.ScopeGlobal:
		return styleBadgeGlobal.Render(text)
	case config.ScopeProject:
		return styleBadgeProject.Render(text)
	case config.ScopeLocal:
		return styleBadgeLocal.Render(text)
	default:
		return ""
	}
}
