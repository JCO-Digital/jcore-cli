package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/JCO-Digital/jcore/container"
)

// Compose files, relative to the project root.
const (
	// GeneratedComposeFile is the compose file jcore writes from its
	// embedded template on every start. It has to be in the project root:
	// compose resolves the relative paths of every file, overrides
	// included, against the directory of the first one.
	GeneratedComposeFile = "compose.jcore.yml"
	// ComposeOverrideFile holds a project's own changes, merged on top of
	// the generated file.
	ComposeOverrideFile = "compose.override.yml"
	// LegacyComposeFile is the compose file projects used to carry
	// themselves, scaffolded from the template and then left to go stale.
	LegacyComposeFile = "docker-compose.yml"
)

// ComposeSetup is the set of compose files a project runs on.
type ComposeSetup struct {
	// Files lists the project's compose files in merge order, without the
	// proxy override.
	Files []string
	// Notice tells the user what became of a docker-compose.yml the
	// project still has, or "" if there is none.
	Notice string
}

// ResolveComposeFiles works out which compose files projectDir runs on:
//
//   - the generated file plus compose.override.yml, if the project has an
//     override;
//   - otherwise its docker-compose.yml, if that has local changes (or no
//     stored checksum to tell), so a customised project keeps working
//     until it has been moved to an override;
//   - otherwise the generated file alone.
//
// docker-compose.yml is never deleted, since a teammate on an older jcore
// may still use it.
func ResolveComposeFiles(projectDir string) (ComposeSetup, error) {
	exists := func(relPath string) bool {
		_, err := os.Stat(filepath.Join(projectDir, relPath))
		return err == nil
	}
	hasLegacy := exists(LegacyComposeFile)

	if exists(ComposeOverrideFile) {
		setup := ComposeSetup{Files: []string{GeneratedComposeFile, ComposeOverrideFile}}
		if hasLegacy {
			setup.Notice = "docker-compose.yml is no longer used, since the project has a compose.override.yml. It can be deleted."
		}
		return setup, nil
	}
	if !hasLegacy {
		return ComposeSetup{Files: []string{GeneratedComposeFile}}, nil
	}

	unmodified, err := CompareChecksum(projectDir, LegacyComposeFile, true)
	if err != nil {
		return ComposeSetup{}, err
	}
	if unmodified {
		return ComposeSetup{
			Files:  []string{GeneratedComposeFile},
			Notice: "docker-compose.yml is no longer used: jcore generates compose.jcore.yml instead. It has no local changes, so it can be deleted.",
		}, nil
	}
	return ComposeSetup{
		Files: []string{LegacyComposeFile},
		Notice: "Warning: docker-compose.yml has local changes, so jcore still uses it instead of the generated compose.jcore.yml.\n" +
			"Updating the project no longer updates docker-compose.yml. Move the project's own changes into\n" +
			"compose.override.yml, then delete docker-compose.yml.",
	}, nil
}

// WriteComposeFile (re)writes the generated compose file to projectDir.
// It's written even while a customised docker-compose.yml is still in
// use, as the reference to compare that one with.
func WriteComposeFile(projectDir string) error {
	return os.WriteFile(filepath.Join(projectDir, GeneratedComposeFile), container.ComposeFile, 0644)
}

// composeFileIgnoreNotice returns a hint to add the generated compose file
// to .gitignore if projectDir is a git repository that doesn't ignore it,
// as is the case for any project older than the file.
func composeFileIgnoreNotice(projectDir string) string {
	cmd := exec.Command("git", "check-ignore", "-q", GeneratedComposeFile)
	cmd.Dir = projectDir
	// Exit code 1 means "not ignored"; anything else (ignored, not a git
	// repository, no git) needs no hint.
	var exitErr *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return "Add /" + GeneratedComposeFile + " to .gitignore: jcore generates it on every start."
	}
	return ""
}
