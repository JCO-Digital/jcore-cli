package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// backgroundCheckEnv marks a process as the detached background checker
// spawned by MaybeCheckInBackground, so it never spawns another one itself.
const backgroundCheckEnv = "JCORE_UPDATE_CHECK_CHILD"

// MaybeCheckInBackground spawns a detached background process to check for
// a new release if the last check is more than CheckInterval old. It never
// blocks: on any error (state unreadable, executable path unresolvable) it
// silently gives up, since an update check must never get in the way of
// the command the user actually ran.
//
// The spawned process runs "jcore update self --check-only", which writes
// the result to the shared state file. It keeps running independently of
// this process, so the result generally isn't visible until the next
// invocation of jcore.
func MaybeCheckInBackground() {
	if os.Getenv(backgroundCheckEnv) != "" {
		return
	}

	state, err := LoadState()
	if err != nil || !state.IsDue() {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer devNull.Close()

	cmd := exec.Command(exePath, "update", "self", "--check-only")
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Env = append(os.Environ(), backgroundCheckEnv+"=1")
	cmd.SysProcAttr = detachSysProcAttr()

	if err := cmd.Start(); err != nil {
		return
	}
	// Detach: don't wait for it, and release our hold on the process so it
	// isn't left as a zombie once it exits.
	_ = cmd.Process.Release()
}

// RunCheckOnly performs the actual release check and persists the result.
// It's what "jcore update self --check-only" runs, whether invoked directly
// or as the detached background process spawned by MaybeCheckInBackground.
func RunCheckOnly(currentVersion string) error {
	latest, _, _, _, err := CheckForUpdate(currentVersion)
	state := State{LastChecked: time.Now()}
	if err == nil {
		state.Latest = latest
	} else {
		// Keep the previously known latest version so the "update
		// available" notice doesn't disappear just because one check
		// failed (e.g. transient network error).
		if prev, loadErr := LoadState(); loadErr == nil {
			state.Latest = prev.Latest
		}
	}
	if saveErr := SaveState(state); saveErr != nil {
		return saveErr
	}
	return err
}

// AvailableNotice returns a one-line message to print if the last
// background check found a newer version than currentVersion, or "" if
// there's nothing to report (never checked, up to date, or the current
// build isn't a released semver version).
func AvailableNotice(currentVersion string) string {
	state, err := LoadState()
	if err != nil || state.Latest == "" {
		return ""
	}

	newer, err := IsNewer(state.Latest, currentVersion)
	if err != nil || !newer {
		return ""
	}

	return "A new version of jcore is available: " + state.Latest +
		" (current: " + currentVersion + "). Run `jcore update self` to update."
}
