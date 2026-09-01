package tui

import (
	"fmt"

	"github.com/JCO-Digital/jcore/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

// editTarget is exactly where an edited value gets written: a scope, and —
// if the value came from a branch override — the specific branch whose
// override table it belongs in.
type editTarget struct {
	scope  config.Scope
	branch string // "" unless writing into a branch-<branch> override table
}

// targetLabel renders target for status messages/hints, e.g. "Project" or
// "Project@staging".
func targetLabel(t editTarget) string {
	label := t.scope.String()
	if t.branch != "" {
		label += "@" + t.branch
	}
	return label
}

// targetFor decides where editing a setting's currently-resolved value
// should write to.
//
// If the value already has an override somewhere (res.IsDefault is
// false), the edit is written back to that exact spot — its scope, and
// its branch override table if that's specifically where it came from.
// Writing anywhere else wouldn't change what's actually in effect: a more
// specific override would still win. This matters in practice — e.g. a
// hand-set or legacy-CLI-written project-level override of a nominally
// global-only setting (CLI Behavior category) needs to be editable right
// where it is, not redirected to Global where the edit would be silently
// shadowed by the untouched project override.
//
// Only a genuinely fresh value (res.IsDefault) picks a scope by category:
// Global for CLI-behavior settings and anything edited outside a project,
// Project otherwise.
func targetFor(def config.SettingDef, res config.Resolution, projectRoot, branch string) editTarget {
	if !res.IsDefault {
		t := editTarget{scope: res.SourceScope}
		if res.FromBranchOverride {
			t.branch = branch
		}
		return t
	}

	if def.ScopeClass == config.ScopeClassGlobalOnly || projectRoot == "" {
		return editTarget{scope: config.ScopeGlobal}
	}
	return editTarget{scope: config.ScopeProject}
}

// afterEditCommitted runs once a new value has been chosen (m.pendingValue
// set) and saves it to its target.
func (m Model) afterEditCommitted() (tea.Model, tea.Cmd) {
	return m.commitSave(targetFor(m.editingDef, m.editingRes, m.projectRoot, m.branch))
}

// commitSave writes m.pendingValue for m.editingDef.Key into target. On
// failure it drops back to the edit screen with an error rather than
// losing the in-progress edit.
//
// Scope validation (the rule that global-only settings can't be written
// at Project scope) only applies when target was freshly chosen by
// category — not when it was read back from an existing override, which
// must remain editable in place even if it sits somewhere that rule
// wouldn't allow for a brand-new value.
func (m Model) commitSave(target editTarget) (tea.Model, tea.Cmd) {
	if m.editingRes.IsDefault {
		if err := config.ValidateScope(m.editingDef.Key, target.scope, m.projectRoot); err != nil {
			m.status = err.Error()
			m.isError = true
			m.state = stateEditing
			return m, nil
		}
	}

	store, err := config.OpenStore(target.scope, m.projectRoot, m.branch)
	if err != nil {
		m.status = err.Error()
		m.isError = true
		m.state = stateEditing
		return m, nil
	}

	if target.branch != "" {
		err = store.SetForBranch(target.branch, m.editingDef.Key, m.pendingValue)
	} else {
		err = store.Set(m.editingDef.Key, m.pendingValue)
	}
	if err != nil {
		m.status = err.Error()
		m.isError = true
		m.state = stateEditing
		return m, nil
	}

	if err := store.Save(); err != nil {
		m.status = err.Error()
		m.isError = true
		m.state = stateEditing
		return m, nil
	}

	m.status = fmt.Sprintf("Saved %s to %s", m.editingDef.Key, targetLabel(target))
	m.isError = false
	m.state = stateBrowsing
	m.refreshItems()
	return m, nil
}
