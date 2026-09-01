package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/JCO-Digital/jcore/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// selectItem points the model's list cursor at the settingItem for key,
// failing the test if it isn't found.
func selectItem(t *testing.T, m *Model, key string) {
	t.Helper()
	for i, it := range m.list.Items() {
		if si, ok := it.(settingItem); ok && si.def.Key == key {
			m.list.Select(i)
			return
		}
	}
	t.Fatalf("no settingItem found for key %q", key)
}

func asModel(t *testing.T, tm tea.Model) Model {
	t.Helper()
	m, ok := tm.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", tm)
	}
	return m
}

func TestEditProjectEligibleSetting_SavesToProject(t *testing.T) {
	root := t.TempDir()
	m := NewModel(root, "")
	selectItem(t, &m, "projectName")

	tm, _ := m.startEdit()
	m = asModel(t, tm)
	if m.state != stateEditing {
		t.Fatalf("state after startEdit = %v, want stateEditing", m.state)
	}

	m.textInput.SetValue("myproject")
	tm, _ = m.updateEditing(tea.KeyMsg{Type: tea.KeyEnter})
	m = asModel(t, tm)

	if m.state != stateBrowsing {
		t.Fatalf("state after committing edit = %v, want stateBrowsing (saved directly, no picker)", m.state)
	}
	if m.isError {
		t.Fatalf("unexpected error status: %s", m.status)
	}

	store, err := config.OpenStore(config.ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	v, ok := store.Get("projectName")
	if !ok || v != "myproject" {
		t.Fatalf("projectName = %#v, %v, want myproject, true", v, ok)
	}
}

func TestEditProjectEligibleSetting_OutsideProject_SavesToGlobal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := NewModel("", "") // no project root
	selectItem(t, &m, "projectName")

	tm, _ := m.startEdit()
	m = asModel(t, tm)
	m.textInput.SetValue("myproject")
	tm, _ = m.updateEditing(tea.KeyMsg{Type: tea.KeyEnter})
	m = asModel(t, tm)

	if m.isError {
		t.Fatalf("unexpected error status: %s", m.status)
	}

	store, err := config.OpenStore(config.ScopeGlobal, "", "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	v, ok := store.Get("projectName")
	if !ok || v != "myproject" {
		t.Fatalf("projectName = %#v, %v, want myproject, true (falls back to Global with no project)", v, ok)
	}
}

func TestEditGlobalOnlySetting_SavesToGlobal(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	m := NewModel(root, "")
	selectItem(t, &m, "mode")

	tm, _ := m.startEdit()
	m = asModel(t, tm)
	if m.state != stateSelecting {
		t.Fatalf("state after startEdit = %v, want stateSelecting (mode declares Options)", m.state)
	}
	if got := m.selectChoices[m.selectCursor]; got != "foreground" {
		t.Fatalf("pre-selected choice = %q, want foreground (the current/default value)", got)
	}

	tm, _ = m.updateSelecting(tea.KeyMsg{Type: tea.KeyDown})
	m = asModel(t, tm)
	tm, _ = m.updateSelecting(tea.KeyMsg{Type: tea.KeyEnter})
	m = asModel(t, tm)

	if m.state != stateBrowsing {
		t.Fatalf("state after committing global-only edit = %v, want stateBrowsing", m.state)
	}
	if m.isError {
		t.Fatalf("unexpected error status: %s", m.status)
	}

	store, err := config.OpenStore(config.ScopeGlobal, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	v, ok := store.Get("mode")
	if !ok || v != "background" {
		t.Fatalf("mode = %#v, %v, want background, true", v, ok)
	}
}

// TestEditSelectSetting_PreservesUnlistedCurrentValue reproduces a setting
// (mode) whose stored value doesn't match either of its declared Options —
// e.g. a legacy or hand-edited value. The select list must still open
// (rather than reject/crash) with that value pre-selected as an extra
// choice, so leaving it alone and pressing enter is a true no-op.
func TestEditSelectSetting_PreservesUnlistedCurrentValue(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	seed, err := config.OpenStore(config.ScopeGlobal, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	if err := seed.Set("mode", "bg"); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := seed.Save(); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	m := NewModel(root, "")
	selectItem(t, &m, "mode")

	tm, _ := m.startEdit()
	m = asModel(t, tm)
	if m.state != stateSelecting {
		t.Fatalf("state after startEdit = %v, want stateSelecting", m.state)
	}
	wantChoices := []string{"bg", "foreground", "background"}
	if !reflect.DeepEqual(m.selectChoices, wantChoices) {
		t.Fatalf("selectChoices = %v, want %v (unlisted current value prepended)", m.selectChoices, wantChoices)
	}
	if got := m.selectChoices[m.selectCursor]; got != "bg" {
		t.Fatalf("pre-selected choice = %q, want bg (the current, unlisted value)", got)
	}

	tm, _ = m.updateSelecting(tea.KeyMsg{Type: tea.KeyEnter})
	m = asModel(t, tm)
	if m.isError {
		t.Fatalf("unexpected error status: %s", m.status)
	}

	store, err := config.OpenStore(config.ScopeGlobal, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := store.Get("mode"); !ok || v != "bg" {
		t.Fatalf("mode = %#v, %v, want bg, true (unchanged)", v, ok)
	}
}

// TestEditGlobalOnlySetting_WithExistingProjectOverride_EditsInPlace
// reproduces a real jcore.toml (from before ValidateScope existed, or from
// the legacy CLI) that already sets "mode" — nominally global-only — at
// project scope. Editing it must update that project override in place:
// writing to Global instead would be silently shadowed by the untouched
// project value, since Project outranks Global in the real merge order.
func TestEditGlobalOnlySetting_WithExistingProjectOverride_EditsInPlace(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	seed, err := config.OpenStore(config.ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	if err := seed.Set("mode", "bg"); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := seed.Save(); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	m := NewModel(root, "")
	selectItem(t, &m, "mode")

	tm, _ := m.startEdit()
	m = asModel(t, tm)
	if m.editingRes.SourceScope != config.ScopeProject {
		t.Fatalf("editingRes.SourceScope = %v, want ScopeProject (that's where the existing value lives)", m.editingRes.SourceScope)
	}
	if m.state != stateSelecting {
		t.Fatalf("state after startEdit = %v, want stateSelecting", m.state)
	}

	// mode's stored value ("bg") isn't one of its declared Options, so it's
	// prepended as an extra, pre-selected choice: ["bg", "foreground",
	// "background"]. Move down twice to pick the real "background" option.
	tm, _ = m.updateSelecting(tea.KeyMsg{Type: tea.KeyDown})
	m = asModel(t, tm)
	tm, _ = m.updateSelecting(tea.KeyMsg{Type: tea.KeyDown})
	m = asModel(t, tm)
	tm, _ = m.updateSelecting(tea.KeyMsg{Type: tea.KeyEnter})
	m = asModel(t, tm)

	if m.isError {
		t.Fatalf("unexpected error status: %s", m.status)
	}

	project, err := config.OpenStore(config.ScopeProject, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := project.Get("mode"); !ok || v != "background" {
		t.Fatalf("project mode = %#v, %v, want background, true (edit should update the existing project override)", v, ok)
	}

	global, err := config.OpenStore(config.ScopeGlobal, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := global.Get("mode"); ok {
		t.Fatal("expected no global mode override to have been created")
	}
}

// TestEditBranchOverriddenSetting_WritesToBranchTable reproduces editing a
// setting whose currently-displayed value comes from a branch override —
// the edit must land in that branch's table, not the file's top level
// (which the branch override would still shadow).
func TestEditBranchOverriddenSetting_WritesToBranchTable(t *testing.T) {
	root := t.TempDir()
	jcoreToml := `remoteDomain = 'kehitys.jcore.fi'

[branch-testing]
remoteDomain = 'stagingtest.jcore.fi'
`
	if err := os.WriteFile(filepath.Join(root, "jcore.toml"), []byte(jcoreToml), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewModel(root, "testing")
	selectItem(t, &m, "remoteDomain")

	tm, _ := m.startEdit()
	m = asModel(t, tm)
	if !m.editingRes.FromBranchOverride {
		t.Fatal("expected editingRes.FromBranchOverride to be true")
	}

	m.textInput.SetValue("newstaging.jcore.fi")
	tm, _ = m.updateEditing(tea.KeyMsg{Type: tea.KeyEnter})
	m = asModel(t, tm)
	if m.isError {
		t.Fatalf("unexpected error status: %s", m.status)
	}

	// On the "testing" branch, the edited value is now in effect.
	onBranch, err := config.OpenStore(config.ScopeProject, root, "testing")
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := onBranch.Get("remoteDomain"); !ok || v != "newstaging.jcore.fi" {
		t.Fatalf("remoteDomain on branch testing = %#v, %v, want newstaging.jcore.fi, true", v, ok)
	}

	// The top-level value (what every other branch sees) is untouched.
	noBranch, err := config.OpenStore(config.ScopeProject, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := noBranch.Get("remoteDomain"); !ok || v != "kehitys.jcore.fi" {
		t.Fatalf("remoteDomain with no branch = %#v, %v, want the untouched top-level value kehitys.jcore.fi, true", v, ok)
	}
}

// TestResetSelected_BranchOverride_ActuallyClearsIt verifies that
// resetting a branch-overridden setting removes the override from its
// real location — the branch table — rather than leaving it in place.
func TestResetSelected_BranchOverride_ActuallyClearsIt(t *testing.T) {
	root := t.TempDir()
	jcoreToml := `remoteDomain = 'kehitys.jcore.fi'

[branch-testing]
remoteDomain = 'stagingtest.jcore.fi'
`
	if err := os.WriteFile(filepath.Join(root, "jcore.toml"), []byte(jcoreToml), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewModel(root, "testing")
	selectItem(t, &m, "remoteDomain")

	tm, _ := m.resetSelected()
	m = asModel(t, tm)
	if m.isError {
		t.Fatalf("unexpected error status: %s", m.status)
	}

	reloaded, err := config.OpenStore(config.ScopeProject, root, "testing")
	if err != nil {
		t.Fatal(err)
	}
	// If the branch override still existed, Get would still return its
	// value ("stagingtest.jcore.fi") instead of falling through to this.
	v, ok := reloaded.Get("remoteDomain")
	if !ok || v != "kehitys.jcore.fi" {
		t.Fatalf("remoteDomain after reset = %#v, %v, want the branch override gone (falls through to the top-level value)", v, ok)
	}
}

func TestToggleBoolSetting(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	m := NewModel(root, "")
	selectItem(t, &m, "wpDebug")

	// wpDebug defaults to true; toggling should flip it to false and save
	// directly (wpDebug is project-eligible, and we're inside a project).
	tm, _ := m.startEdit()
	m = asModel(t, tm)
	if m.state != stateBrowsing {
		t.Fatalf("state after toggling bool = %v, want stateBrowsing (saved directly, no picker)", m.state)
	}
	if m.isError {
		t.Fatalf("unexpected error status: %s", m.status)
	}

	store, err := config.OpenStore(config.ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	v, ok := store.Get("wpDebug")
	if !ok || v != false {
		t.Fatalf("wpDebug = %#v, %v, want false, true", v, ok)
	}
}

func TestResetSelected_RemovesOverride(t *testing.T) {
	root := t.TempDir()

	store, err := config.OpenStore(config.ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	_ = store.Set("theme", "custom-theme")
	if err := store.Save(); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	m := NewModel(root, "")
	selectItem(t, &m, "theme")

	tm, _ := m.resetSelected()
	m = asModel(t, tm)
	if m.isError {
		t.Fatalf("unexpected error status: %s", m.status)
	}

	reloaded, err := config.OpenStore(config.ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore (reload) error = %v", err)
	}
	if _, ok := reloaded.Get("theme"); ok {
		t.Fatal("expected theme override to be removed")
	}

	// Resetting an already-default setting is a no-op with a status
	// message, not an error.
	selectItem(t, &m, "theme")
	tm, _ = m.resetSelected()
	m = asModel(t, tm)
	if m.isError {
		t.Fatalf("expected no-op reset to not be an error, got: %s", m.status)
	}
}

// TestProgramRunsAndQuits drives the model through a real bubbletea
// program (headless, via teatest) rather than calling Update directly, to
// exercise the actual Init/Update/View wiring: real key messages routed
// through list navigation, a full render pass, and a clean shutdown on "q".
func TestProgramRunsAndQuits(t *testing.T) {
	root := t.TempDir()
	tm := teatest.NewTestModel(t, NewModel(root, ""), teatest.WithInitialTermSize(100, 30))

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestBuildItems_ShowsBranchOverride(t *testing.T) {
	root := t.TempDir()

	jcoreToml := `remoteDomain = 'kehitys.jcore.fi'

[branch-testing]
remoteDomain = 'stagingtest.jcore.fi'
`
	if err := os.WriteFile(filepath.Join(root, "jcore.toml"), []byte(jcoreToml), 0644); err != nil {
		t.Fatal(err)
	}

	items := buildItems(root, "testing")
	var found *settingItem
	for _, it := range items {
		if si, ok := it.(settingItem); ok && si.def.Key == "remoteDomain" {
			found = &si
			break
		}
	}
	if found == nil {
		t.Fatal("remoteDomain not found in buildItems result")
	}
	if !found.resolution.FromBranchOverride || found.resolution.Value != "stagingtest.jcore.fi" {
		t.Fatalf("remoteDomain resolution = %+v, want FromBranchOverride with stagingtest.jcore.fi", found.resolution)
	}
	if badge := scopeBadge(found.resolution, "testing"); badge == "" {
		t.Fatal("expected a non-empty badge")
	}
}

func TestEscapeCancelsEditWithoutSaving(t *testing.T) {
	root := t.TempDir()
	m := NewModel(root, "")
	selectItem(t, &m, "projectName")

	tm, _ := m.startEdit()
	m = asModel(t, tm)
	m.textInput.SetValue("should-not-be-saved")

	tm, _ = m.updateEditing(tea.KeyMsg{Type: tea.KeyEscape})
	m = asModel(t, tm)
	if m.state != stateBrowsing {
		t.Fatalf("state after esc = %v, want stateBrowsing", m.state)
	}

	store, err := config.OpenStore(config.ScopeProject, root, "")
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	if _, ok := store.Get("projectName"); ok {
		t.Fatal("expected no value to be saved after cancelling with esc")
	}
}
