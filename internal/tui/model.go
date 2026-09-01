// Package tui implements the "jcore config edit" interactive settings
// editor: a scrollable, filterable list of every known jcore setting, its
// current effective value, and which scope (and branch override, if any)
// that value comes from, with an in-place edit flow that writes each edit
// back to that exact same spot — or, for a setting with no existing
// override anywhere, to its default scope (Global for CLI-behavior
// settings, Project otherwise) — without ever prompting.
package tui

import (
	"fmt"

	"github.com/JCO-Digital/jcore/internal/config"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type uiState int

const (
	stateBrowsing uiState = iota
	stateEditing
	stateSelecting
)

// Model is the top-level bubbletea model for the config editor.
type Model struct {
	list        list.Model
	state       uiState
	projectRoot string
	branch      string

	editingDef   config.SettingDef
	editingRes   config.Resolution
	pendingValue any
	textInput    textinput.Model

	// selectChoices/selectCursor back stateSelecting, used instead of
	// textInput for settings with a fixed SettingDef.Options list.
	selectChoices []string
	selectCursor  int

	status  string
	isError bool
}

// NewModel builds the editor's initial state. projectRoot may be "" when
// not run from inside a jcore project, in which case only global-scope
// edits are offered. branch is the currently checked-out git branch (see
// project.CurrentBranch); pass "" to disable branch-override display/edits.
func NewModel(projectRoot, branch string) Model {
	l := list.New(buildItems(projectRoot, branch), rowDelegate{branch: branch}, 0, 0)
	l.Title = "jcore configuration"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = styleTitle

	return Model{
		list:        l,
		state:       stateBrowsing,
		projectRoot: projectRoot,
		branch:      branch,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h := msg.Height - 1 // leave a line for the status bar
		if h < 0 {
			h = 0
		}
		m.list.SetSize(msg.Width, h)
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case stateEditing:
			return m.updateEditing(msg)
		case stateSelecting:
			return m.updateSelecting(msg)
		default:
			return m.updateBrowsing(msg)
		}
	}
	return m, nil
}

func (m Model) updateBrowsing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While the user is actively typing a filter query, let the list
	// consume every key (including ones we'd otherwise bind, like "q").
	if m.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "enter":
		return m.startEdit()
	case "x":
		return m.resetSelected()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	switch m.state {
	case stateEditing:
		return m.editView()
	case stateSelecting:
		return m.selectView()
	default:
		view := m.list.View()
		if m.status != "" {
			if m.isError {
				view += "\n" + styleError.Render(m.status)
			} else {
				view += "\n" + styleStatus.Render(m.status)
			}
		} else {
			view += "\n" + styleHelp.Render("enter edit · x reset to default · / filter · q quit")
		}
		return view
	}
}

// startEdit begins editing the currently selected setting. Bool settings
// are toggled immediately (no text-entry sub-screen needed); everything
// else opens a text input pre-filled with the current value.
func (m Model) startEdit() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(settingItem)
	if !ok {
		return m, nil // a category header is selected; nothing to edit
	}

	m.editingDef = item.def
	m.editingRes = item.resolution
	m.status = ""
	m.isError = false

	if item.def.Type == config.TypeBool {
		current, _ := item.resolution.Value.(bool)
		m.pendingValue = !current
		return m.afterEditCommitted()
	}

	if len(item.def.Options) > 0 {
		return m.startSelect(item)
	}

	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 60
	ti.Focus()
	if item.def.Sensitive {
		ti.EchoMode = textinput.EchoPassword
	}
	ti.SetValue(formatForEdit(item.def, item.resolution.Value))
	ti.CursorEnd()
	m.textInput = ti
	m.state = stateEditing
	return m, nil
}

func (m Model) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateBrowsing
		m.status = "Cancelled"
		m.isError = false
		return m, nil
	case "enter":
		m.pendingValue = m.textInput.Value()
		return m.afterEditCommitted()
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) editView() string {
	view := styleTitle.Render("Edit "+m.editingDef.Key) + "\n\n"
	if m.editingDef.Description != "" {
		view += styleDesc.Render(m.editingDef.Description) + "\n\n"
	}
	view += "  " + m.textInput.View() + "\n\n"
	if m.editingDef.Type == config.TypeStringSlice {
		view += styleHelp.Render("comma-separated list") + "\n"
	}
	if m.status != "" && m.isError {
		view += styleError.Render(m.status) + "\n"
	}
	view += styleHelp.Render(fmt.Sprintf("enter save to %s · esc cancel", targetLabel(targetFor(m.editingDef, m.editingRes, m.projectRoot, m.branch))))
	return view
}

// resetSelected removes the currently selected setting's override from
// whichever scope its effective value is actually sourced from.
func (m Model) resetSelected() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(settingItem)
	if !ok {
		return m, nil
	}
	if item.resolution.IsDefault {
		m.status = fmt.Sprintf("%s is already at its default value", item.def.Key)
		m.isError = false
		return m, nil
	}

	target := editTarget{scope: item.resolution.SourceScope}
	if item.resolution.FromBranchOverride {
		target.branch = m.branch
	}

	store, err := config.OpenStore(target.scope, m.projectRoot, m.branch)
	if err != nil {
		m.status = err.Error()
		m.isError = true
		return m, nil
	}

	if target.branch != "" {
		err = store.UnsetForBranch(target.branch, item.def.Key)
	} else {
		err = store.Unset(item.def.Key)
	}
	if err != nil {
		m.status = err.Error()
		m.isError = true
		return m, nil
	}
	if err := store.Save(); err != nil {
		m.status = err.Error()
		m.isError = true
		return m, nil
	}

	m.status = fmt.Sprintf("Removed %s override from %s", item.def.Key, targetLabel(target))
	m.isError = false
	m.refreshItems()
	return m, nil
}

// refreshItems rebuilds every row's resolved value/scope after a save or
// reset, preserving the current cursor position.
func (m *Model) refreshItems() {
	index := m.list.Index()
	m.list.SetItems(buildItems(m.projectRoot, m.branch))
	m.list.Select(index)
}
