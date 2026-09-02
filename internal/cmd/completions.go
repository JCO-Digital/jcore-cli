package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// completionTargets are the shells update self installs completions for,
// and where it puts them. These paths must match the Makefile's
// install-completions target so a self-updated binary and a `make install`
// agree on where completions live.
var completionTargets = []struct {
	shell string
	path  []string // joined onto $HOME
}{
	{"bash", []string{".local", "share", "bash-completion", "completions", "jcore"}},
	{"zsh", []string{".local", "share", "zsh", "site-functions", "_jcore"}},
	{"fish", []string{".config", "fish", "completions", "jcore.fish"}},
}

// installShellCompletions (re)generates and writes bash/zsh/fish completion
// scripts by invoking `<exePath> completion <shell>`, so the scripts always
// match whatever binary is at exePath rather than the possibly-older
// in-memory command tree of the process doing the installing. It's
// best-effort per shell: a failure for one (e.g. no write permission) is
// reported but doesn't stop the others.
func installShellCompletions(exePath string) (installed, failed []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		for _, t := range completionTargets {
			failed = append(failed, t.shell)
		}
		return installed, failed
	}

	for _, t := range completionTargets {
		path := filepath.Join(append([]string{home}, t.path...)...)

		out, err := exec.Command(exePath, "completion", t.shell).Output()
		if err != nil {
			failed = append(failed, t.shell)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			failed = append(failed, t.shell)
			continue
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			failed = append(failed, t.shell)
			continue
		}
		installed = append(installed, t.shell)
	}

	return installed, failed
}

// reportCompletions prints a one-line summary of an installShellCompletions
// result. Failures are informational, not fatal: completions are a
// convenience, not something worth failing "update self" over.
func reportCompletions(installed, failed []string) {
	if len(installed) > 0 {
		fmt.Printf("Installed shell completions: %s\n", strings.Join(installed, ", "))
	}
	if len(failed) > 0 {
		fmt.Printf("Could not install completions for: %s\n", strings.Join(failed, ", "))
	}
}
