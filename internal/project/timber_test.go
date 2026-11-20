package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestTimberConstraintForBranch(t *testing.T) {
	for branch, want := range map[string]string{
		"timber2/grasshopper": "^2",
		"timber2/elduelo":     "^2",
		"grasshopper":         "^1",
		"":                    "^1",
	} {
		if got := TimberConstraintForBranch(branch); got != want {
			t.Errorf("TimberConstraintForBranch(%q) = %q, want %q", branch, got, want)
		}
	}
}

func TestThemeArchiveBranch(t *testing.T) {
	for branch, want := range map[string]string{
		"timber2/grasshopper": "grasshopper",
		"grasshopper":         "grasshopper",
		"irishcoffee":         "irishcoffee",
	} {
		if got := ThemeArchiveBranch(branch); got != want {
			t.Errorf("ThemeArchiveBranch(%q) = %q, want %q", branch, got, want)
		}
	}
}

func TestResolveTimberVersion(t *testing.T) {
	writeTimber := func(t *testing.T, dir, constraint string) {
		t.Helper()
		content := `{"require": {"timber/timber": "` + constraint + `"}}`
		if err := os.WriteFile(filepath.Join(dir, "composer.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name     string
		branch   string // project's branch setting
		existing string // composer.json timber constraint; "" = none
		want     string
	}{
		{name: "new project gets the default branch's Timber", want: "^2"},
		{name: "timber1 branch stays on ^1", branch: "grasshopper", existing: "^1", want: "^1"},
		{name: "switching to a timber2 branch moves to ^2", branch: "timber2/grasshopper", existing: "^1", want: "^2"},
		{name: "no branch setting keeps composer.json's constraint", existing: "^1", want: "^1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			if tt.branch != "" {
				viper.Set("branch", tt.branch)
			}
			dir := t.TempDir()
			if tt.existing != "" {
				writeTimber(t, dir, tt.existing)
			}
			if got := ResolveTimberVersion(dir, "timber2/grasshopper"); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJcore2CatalogDefaultsToTimber2(t *testing.T) {
	catalog, err := LoadTemplateCatalog()
	if err != nil {
		t.Fatal(err)
	}
	entry := catalog["jcore2"]
	if !strings.HasPrefix(entry.Branch, Timber2BranchPrefix) {
		t.Errorf("jcore2 default branch %q is not a Timber 2 branch", entry.Branch)
	}
	for _, b := range entry.Branches {
		if !strings.HasPrefix(b, Timber2BranchPrefix) {
			t.Errorf("jcore2 offers non-Timber 2 branch %q to new projects", b)
		}
	}
	if len(entry.Submodules) != 1 || entry.Submodules[0].Path != "wp-content/themes/jcore2" || !entry.Submodules[0].UseBranch {
		t.Errorf("unexpected jcore2 submodules: %+v", entry.Submodules)
	}
}

func TestScaffoldJcore2TimberVersion(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("projectName", "test-project")
	viper.Set("template", "jcore2")
	viper.Set("branch", "timber2/grasshopper")

	dir := t.TempDir()
	if err := ScaffoldProject(dir, "jcore2"); err != nil {
		t.Fatalf("ScaffoldProject failed: %v", err)
	}
	if got := composerRequire(dir, "timber/timber"); got != "^2" {
		t.Errorf("new jcore2 project requires timber %q, want ^2", got)
	}

	// An older project on a Timber 1 branch keeps ^1 when updated.
	viper.Set("branch", "grasshopper")
	if err := UpdateProject(dir, nil, nil); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}
	if got := composerRequire(dir, "timber/timber"); got != "^1" {
		t.Errorf("timber1-branch project requires timber %q after update, want ^1", got)
	}
}

func TestAddTemplateSubmodules(t *testing.T) {
	// Newer git refuses file:// submodules unless allowed.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")

	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "init.defaultBranch=main"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	theme := t.TempDir()
	git(theme, "init")
	git(theme, "commit", "--allow-empty", "-m", "init")
	git(theme, "branch", "timber2/grasshopper")

	project := t.TempDir()
	git(project, "init")

	subs := []TemplateSubmodule{{Path: "wp-content/themes/jcore2", Repo: theme, UseBranch: true}}
	if err := AddTemplateSubmodules(project, subs, "timber2/grasshopper"); err != nil {
		t.Fatalf("AddTemplateSubmodules failed: %v", err)
	}

	gitmodules, err := os.ReadFile(filepath.Join(project, ".gitmodules"))
	if err != nil {
		t.Fatalf("no .gitmodules: %v", err)
	}
	if !strings.Contains(string(gitmodules), "branch = timber2/grasshopper") {
		t.Errorf("submodule doesn't track the branch:\n%s", gitmodules)
	}

	out, err := exec.Command("git", "-C", filepath.Join(project, "wp-content/themes/jcore2"), "branch", "--show-current").Output()
	if err != nil || strings.TrimSpace(string(out)) != "timber2/grasshopper" {
		t.Errorf("submodule checked out %q (err %v), want timber2/grasshopper", strings.TrimSpace(string(out)), err)
	}
}
