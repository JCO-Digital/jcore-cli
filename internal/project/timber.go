package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Timber2BranchPrefix marks the jcore2 parent theme branches built on
// Timber 2 ("timber2/grasshopper"). The bare branches ("grasshopper") are
// the original Timber 1 ones, which existing projects keep tracking.
const Timber2BranchPrefix = "timber2/"

// TimberConstraintForBranch returns the timber/timber composer constraint a
// jcore2 parent theme branch needs.
func TimberConstraintForBranch(branch string) string {
	if strings.HasPrefix(branch, Timber2BranchPrefix) {
		return "^2"
	}
	return "^1"
}

// composerRequire returns the constraint the project's composer.json
// requires for pkg, or "" if the file or requirement is missing.
func composerRequire(projectDir, pkg string) string {
	raw, err := os.ReadFile(filepath.Join(projectDir, "composer.json"))
	if err != nil {
		return ""
	}
	var composer struct {
		Require map[string]string `json:"require"`
	}
	if err := json.Unmarshal(raw, &composer); err != nil {
		return ""
	}
	return composer.Require[pkg]
}

// ResolveTimberVersion returns the timber/timber constraint for the jcore2
// composer template. A project's own branch setting decides it. Without one
// (older projects never stored it), the constraint already in the project's
// composer.json is kept, so updating such a project never pulls in Timber 2;
// only a brand-new project falls back to the template's default branch.
func ResolveTimberVersion(projectDir, defaultBranch string) string {
	if branch := viper.GetString("branch"); branch != "" {
		return TimberConstraintForBranch(branch)
	}
	if existing := composerRequire(projectDir, "timber/timber"); existing != "" {
		return existing
	}
	return TimberConstraintForBranch(defaultBranch)
}

// ThemeArchiveBranch returns the branch name used in a child theme archive
// URL. The jcore2 child theme is the same for a parent branch on either
// Timber version, so "timber2/grasshopper" downloads the "grasshopper"
// archive (and a slash would not fit in its file name anyway).
func ThemeArchiveBranch(branch string) string {
	return strings.TrimPrefix(branch, Timber2BranchPrefix)
}
