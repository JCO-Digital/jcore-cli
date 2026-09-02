package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// startSelect begins editing a setting that declares a fixed set of known
// SettingDef.Options via an up/down selectable list, instead of the
// free-text textinput sub-screen. If the setting's current value isn't one
// of Options (e.g. a hand-edited value, or one written under an older,
// different set of known values), it's prepended as an extra choice and
// pre-selected, so it stays intact unless the user actively picks a listed
// option instead.
func (m Model) startSelect(item settingItem) (tea.Model, tea.Cmd) {
	current := formatForEdit(item.def, item.resolution.Value)

	choices := item.def.Options
	cursor := indexOf(choices, current)
	if cursor == -1 {
		if current != "" {
			choices = append([]string{current}, choices...)
		}
		cursor = 0
	}

	m.selectChoices = choices
	m.selectCursor = cursor
	m.state = stateSelecting
	return m, nil
}

func indexOf(items []string, target string) int {
	for i, v := range items {
		if v == target {
			return i
		}
	}
	return -1
}

func (m Model) updateSelecting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateBrowsing
		m.status = "Cancelled"
		m.isError = false
		return m, nil
	case "up", "k":
		if m.selectCursor > 0 {
			m.selectCursor--
		}
		return m, nil
	case "down", "j":
		if m.selectCursor < len(m.selectChoices)-1 {
			m.selectCursor++
		}
		return m, nil
	case "enter":
		m.pendingValue = m.selectChoices[m.selectCursor]
		return m.afterEditCommitted()
	}
	return m, nil
}

func (m Model) selectView() string {
	view := styleTitle.Render("Edit "+m.editingDef.Key) + "\n\n"
	if m.editingDef.Description != "" {
		view += styleDesc.Render(m.editingDef.Description) + "\n\n"
	}

	for i, choice := range m.selectChoices {
		line := choice
		if line == "" {
			line = "(empty)"
		}
		if i == m.selectCursor {
			view += styleSelected.Render("> "+line) + "\n"
		} else {
			view += "  " + line + "\n"
		}
	}

	view += "\n"
	if m.status != "" && m.isError {
		view += styleError.Render(m.status) + "\n"
	}
	view += styleHelp.Render(fmt.Sprintf("enter save to %s · esc cancel", targetLabel(targetFor(m.editingDef, m.editingRes, m.projectRoot, m.branch))))
	return view
}
