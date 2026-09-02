package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CheckInterval is how often the background update check is allowed to run.
// Every command invocation checks the saved state cheaply; only once per
// interval does it actually spawn a background process to hit the network.
const CheckInterval = 24 * time.Hour

// State is the persisted result of the last background update check.
type State struct {
	// Latest is the newest release tag seen, e.g. "v3.17.0".
	Latest string `json:"latest"`
	// LastChecked is when the check last ran (successfully or not).
	LastChecked time.Time `json:"lastChecked"`
}

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jcore", "update-state.json"), nil
}

// LoadState reads the persisted update-check state. A missing file is not
// an error; it just yields a zero State, meaning "never checked".
func LoadState() (State, error) {
	var state State

	path, err := statePath()
	if err != nil {
		return state, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

// SaveState persists the update-check state, creating the config directory
// if needed.
func SaveState(state State) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "\t")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// IsDue reports whether enough time has passed since the last check that a
// new one should run.
func (s State) IsDue() bool {
	return time.Since(s.LastChecked) > CheckInterval
}
