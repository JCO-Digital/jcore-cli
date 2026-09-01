package project

import (
	"os/exec"
	"testing"
)

func TestCurrentBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	if got := CurrentBranch(""); got != "" {
		t.Fatalf("CurrentBranch(\"\") = %q, want \"\"", got)
	}

	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "feature-branch")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "init")

	if got := CurrentBranch(dir); got != "feature-branch" {
		t.Fatalf("CurrentBranch(dir) = %q, want feature-branch", got)
	}

	if got := CurrentBranch(t.TempDir()); got != "" {
		t.Fatalf("CurrentBranch(non-git dir) = %q, want \"\"", got)
	}
}
