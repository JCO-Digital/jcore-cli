package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JCO-Digital/jcore/container"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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
//   - the generated file, plus compose.override.yml if there is one, if
//     the project has an override or the mounts setting;
//   - otherwise its docker-compose.yml, if that has local changes (or no
//     stored checksum to tell), so a customised project keeps working
//     until it has been converted to mounts or an override;
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
	hasOverride := exists(ComposeOverrideFile)

	if hasOverride || len(viper.GetStringSlice("mounts")) > 0 {
		setup := ComposeSetup{Files: []string{GeneratedComposeFile}}
		if hasOverride {
			setup.Files = append(setup.Files, ComposeOverrideFile)
		}
		if hasLegacy {
			setup.Notice = "docker-compose.yml is no longer used, since the project has the mounts setting or a compose.override.yml. It can be deleted."
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
			"Updating the project no longer updates docker-compose.yml. Move extra bind mounts into the mounts\n" +
			"setting (jcore.toml), anything else into compose.override.yml, then delete docker-compose.yml.",
	}, nil
}

// mountServices are the services the mounts setting adds its bind mounts
// to: nginx serves static files from the webroot and PHP-FPM runs the
// code, so a mount usually needs to be in both.
var mountServices = []string{"wordpress", "web"}

// WriteComposeFile (re)writes the generated compose file to projectDir,
// with the mounts setting added. It's written even while a customised
// docker-compose.yml is still in use, as the reference to compare that one
// with.
func WriteComposeFile(projectDir string) error {
	content, err := renderComposeFile(viper.GetStringSlice("mounts"))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectDir, GeneratedComposeFile), content, 0644)
}

// renderComposeFile returns the embedded compose file with mounts added to
// the volumes of each of mountServices. Without mounts it's the embedded
// file as is.
func renderComposeFile(mounts []string) ([]byte, error) {
	if len(mounts) == 0 {
		return container.ComposeFile, nil
	}
	for _, mount := range mounts {
		if err := validateMount(mount); err != nil {
			return nil, err
		}
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(container.ComposeFile, &doc); err != nil {
		return nil, err
	}
	services := mappingValue(doc.Content[0], "services")
	for _, name := range mountServices {
		volumes := mappingValue(mappingValue(services, name), "volumes")
		if volumes == nil || volumes.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("the compose template has no volumes list for %s", name)
		}
		for _, mount := range mounts {
			volumes.Content = append(volumes.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: mount})
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// validateMount checks a mounts entry is a "<source>:<target>[:<mode>]"
// bind mount. A source that isn't a path would make compose create a
// named volume instead, silently.
func validateMount(mount string) error {
	source, target, ok := strings.Cut(mount, ":")
	switch {
	case !ok || source == "" || target == "":
		return fmt.Errorf("mounts entry %q isn't in the form \"./source:/target\"", mount)
	case !strings.HasPrefix(source, ".") && !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, "~"):
		return fmt.Errorf("mounts entry %q: the source must be a path (./, ../, / or ~), relative to the project root", mount)
	case !strings.HasPrefix(target, "/"):
		return fmt.Errorf("mounts entry %q: the target must be an absolute path in the container", mount)
	}
	return nil
}

// mappingValue returns the value node for key in the mapping node m, or nil.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
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
