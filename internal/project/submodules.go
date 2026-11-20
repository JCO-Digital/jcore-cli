package project

import (
	"fmt"
	"os/exec"
)

// TemplateSubmodule is a git submodule a template adds to new projects,
// such as the jcore2 parent theme.
type TemplateSubmodule struct {
	Path string `toml:"path"`
	Repo string `toml:"repo"`
	// UseBranch makes the submodule track the project's branch setting.
	UseBranch bool `toml:"useBranch"`
}

// AddTemplateSubmodules adds the template's submodules to the freshly
// git-initialised project in projectDir, tracking branch where the
// submodule asks for it, mirroring the legacy CLI's createProject(). Every
// submodule is attempted; the first failure is returned.
func AddTemplateSubmodules(projectDir string, submodules []TemplateSubmodule, branch string) error {
	var firstErr error
	for _, sub := range submodules {
		args := []string{"submodule", "add", "-f"}
		if sub.UseBranch && branch != "" {
			args = append(args, "-b", branch)
		}
		args = append(args, sub.Repo, sub.Path)

		cmd := exec.Command("git", args...)
		cmd.Dir = projectDir
		if out, err := cmd.CombinedOutput(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("git submodule add %s: %w\n%s", sub.Path, err, out)
		}
	}
	return firstErr
}
