package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildTestBinary compiles the jcore binary once for use as the "exePath"
// installShellCompletions shells out to, so the test exercises the real
// `completion <shell>` output rather than a fake stand-in.
func buildTestBinary(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "jcore-test-bin")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/JCO-Digital/jcore/cmd/jcore")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test binary: %v\n%s", err, out)
	}
	return bin
}

func TestInstallShellCompletions(t *testing.T) {
	bin := buildTestBinary(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	installed, failed := installShellCompletions(bin)

	if len(failed) != 0 {
		t.Fatalf("installShellCompletions() failed = %v, want none", failed)
	}
	if len(installed) != len(completionTargets) {
		t.Fatalf("installShellCompletions() installed = %v, want all of %v", installed, completionTargets)
	}

	for _, target := range completionTargets {
		path := filepath.Join(append([]string{home}, target.path...)...)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected completion file for %s at %s: %v", target.shell, path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("completion file for %s at %s is empty", target.shell, path)
		}
	}
}

func TestInstallShellCompletionsBadExecutable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	installed, failed := installShellCompletions("/nonexistent/binary")

	if len(installed) != 0 {
		t.Fatalf("installShellCompletions() with bad exePath installed = %v, want none", installed)
	}
	if len(failed) != len(completionTargets) {
		t.Fatalf("installShellCompletions() with bad exePath failed = %v, want all shells", failed)
	}
}
