package update

import (
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	loaded, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() on missing file error = %v", err)
	}
	if loaded != (State{}) {
		t.Fatalf("LoadState() on missing file = %+v, want zero value", loaded)
	}

	want := State{Latest: "v3.17.0", LastChecked: time.Now().Truncate(time.Second)}
	if err := SaveState(want); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	got, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if !got.LastChecked.Equal(want.LastChecked) || got.Latest != want.Latest {
		t.Fatalf("LoadState() = %+v, want %+v", got, want)
	}
}

func TestStateIsDue(t *testing.T) {
	fresh := State{LastChecked: time.Now()}
	if fresh.IsDue() {
		t.Fatal("freshly checked state should not be due")
	}

	stale := State{LastChecked: time.Now().Add(-2 * CheckInterval)}
	if !stale.IsDue() {
		t.Fatal("stale state should be due")
	}

	if !(State{}).IsDue() {
		t.Fatal("never-checked (zero) state should be due")
	}
}

func TestAvailableNotice(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if notice := AvailableNotice("3.16.3"); notice != "" {
		t.Fatalf("AvailableNotice() with no state = %q, want empty", notice)
	}

	if err := SaveState(State{Latest: "v3.17.0", LastChecked: time.Now()}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	if notice := AvailableNotice("3.16.3"); notice == "" {
		t.Fatal("AvailableNotice() = empty, want a notice for an older current version")
	}

	if err := SaveState(State{Latest: "v3.16.3", LastChecked: time.Now()}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	if notice := AvailableNotice("3.16.3"); notice != "" {
		t.Fatalf("AvailableNotice() = %q, want empty when already current", notice)
	}
}
