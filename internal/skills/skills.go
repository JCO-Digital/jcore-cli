// Package skills keeps a local checkout of the JCO-Digital/jcore-skills
// repository up to date and links its skills into Claude Code's global
// skills directory.
package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultRepo is the git URL of the shared JCORE agent skills.
const DefaultRepo = "https://github.com/JCO-Digital/jcore-skills.git"

// CheckoutDir is where jcore keeps its managed clone of the skills repo:
// $XDG_DATA_HOME/jcore/jcore-skills, defaulting to
// ~/.local/share/jcore/jcore-skills.
func CheckoutDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "jcore", "jcore-skills"), nil
}

// ClaudeSkillsDir is Claude Code's global skills directory:
// $CLAUDE_CONFIG_DIR/skills, defaulting to ~/.claude/skills.
func ClaudeSkillsDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "skills"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "skills"), nil
}

// Fetch clones repo into dir, or, if dir is already a clone, fetches the
// remote's default branch and hard-resets onto it. The checkout is owned by
// jcore, so local edits in it are discarded.
func Fetch(repo, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		return git("", "clone", "--depth", "1", repo, dir)
	}
	if err := git(dir, "remote", "set-url", "origin", repo); err != nil {
		return err
	}
	if err := git(dir, "fetch", "--depth", "1", "origin", "HEAD"); err != nil {
		return err
	}
	return git(dir, "reset", "--hard", "--quiet", "FETCH_HEAD")
}

func git(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Discover lists the skills in a checkout: the names of its top-level
// directories that contain a SKILL.md.
func Discover(checkout string) ([]string, error) {
	entries, err := os.ReadDir(checkout)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(checkout, e.Name(), "SKILL.md")); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Status is what Install did with one skill.
type Status string

const (
	Linked   Status = "linked"     // new symlink created
	UpToDate Status = "up to date" // symlink already pointed at the checkout
	Replaced Status = "replaced"   // a symlink to elsewhere was replaced (force)
	Skipped  Status = "skipped"    // something else is in the way
	Removed  Status = "removed"    // stale link to a skill no longer upstream
)

// Result reports the outcome for one skill name.
type Result struct {
	Name   string
	Status Status
	Detail string
}

// Install symlinks every skill in checkout into target. A name already
// taken by a symlink pointing somewhere else (e.g. a developer's own clone)
// is left alone unless force is set; a real file or directory is never
// touched. Symlinks in target that point into checkout at a skill that no
// longer exists upstream are removed.
func Install(checkout, target string, force bool) ([]Result, error) {
	names, err := Discover(checkout)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, err
	}

	var results []Result
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
		results = append(results, installOne(filepath.Join(checkout, name), filepath.Join(target, name), name, force))
	}

	stale, err := pruneStale(checkout, target, wanted)
	if err != nil {
		return results, err
	}
	return append(results, stale...), nil
}

func installOne(src, dst, name string, force bool) Result {
	info, err := os.Lstat(dst)
	if os.IsNotExist(err) {
		if err := os.Symlink(src, dst); err != nil {
			return Result{name, Skipped, err.Error()}
		}
		return Result{name, Linked, ""}
	}
	if err != nil {
		return Result{name, Skipped, err.Error()}
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return Result{name, Skipped, dst + " exists and is not a symlink; remove it to install the shared version"}
	}

	current, err := os.Readlink(dst)
	if err != nil {
		return Result{name, Skipped, err.Error()}
	}
	if current == src {
		return Result{name, UpToDate, ""}
	}
	if !force {
		return Result{name, Skipped, "already links to " + current + " (use --force to replace)"}
	}
	if err := os.Remove(dst); err != nil {
		return Result{name, Skipped, err.Error()}
	}
	if err := os.Symlink(src, dst); err != nil {
		return Result{name, Skipped, err.Error()}
	}
	return Result{name, Replaced, "was " + current}
}

func pruneStale(checkout, target string, wanted map[string]bool) ([]Result, error) {
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	prefix := filepath.Clean(checkout) + string(filepath.Separator)
	var results []Result
	for _, e := range entries {
		if wanted[e.Name()] || e.Type()&os.ModeSymlink == 0 {
			continue
		}
		dst := filepath.Join(target, e.Name())
		link, err := os.Readlink(dst)
		if err != nil || !strings.HasPrefix(link, prefix) {
			continue
		}
		if err := os.Remove(dst); err != nil {
			results = append(results, Result{e.Name(), Skipped, err.Error()})
			continue
		}
		results = append(results, Result{e.Name(), Removed, "no longer in the skills repo"})
	}
	return results, nil
}
