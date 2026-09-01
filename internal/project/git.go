package project

import (
	"os/exec"
	"strings"
)

// CurrentBranch returns the git branch currently checked out in root, or
// "" if root is empty, isn't a git repository, or the branch can't be
// determined (e.g. detached HEAD, git not installed). It never errors:
// branch-specific config overrides are an optional convenience, not
// something that should block any other command from running.
func CurrentBranch(root string) string {
	if root == "" {
		return ""
	}
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
