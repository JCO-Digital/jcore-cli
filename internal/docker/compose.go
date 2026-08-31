package docker

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

// ComposeUp runs docker compose up
func ComposeUp(projectDir string, detached bool) error {
	args := []string{"compose", "up"}
	if detached {
		args = append(args, "-d")
	}

	cmd := exec.Command("docker", args...)
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// ComposeStop runs docker compose stop
func ComposeStop(projectDir string) error {
	cmd := exec.Command("docker", "compose", "stop")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// ComposeLogs runs docker compose logs -f, optionally scoped to a service
func ComposeLogs(projectDir string, service string) error {
	args := []string{"compose", "logs", "-f", "--since", "5m"}
	if service != "" {
		args = append(args, service)
	}

	cmd := exec.Command("docker", args...)
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// ComposeRm runs docker compose rm -f
func ComposeRm(projectDir string) error {
	cmd := exec.Command("docker", "compose", "rm", "-f")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// VolumesByProject lists volume names labeled for a given compose project
func VolumesByProject(projectName string) ([]string, error) {
	out, err := exec.Command("docker", "volume", "ls", "-q", "--filter=label=com.docker.compose.project="+projectName).Output()
	if err != nil {
		return nil, err
	}

	var volumes []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			volumes = append(volumes, line)
		}
	}

	return volumes, nil
}

// VolumeRm removes a docker volume
func VolumeRm(name string) error {
	cmd := exec.Command("docker", "volume", "rm", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// PruneContainers removes stopped containers
func PruneContainers() error {
	cmd := exec.Command("docker", "container", "prune", "-f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// PruneImages removes unused images. If all is true, unused (not just dangling) images are removed too.
func PruneImages(all bool) error {
	args := []string{"image", "prune", "-f"}
	if all {
		args = append(args, "-a")
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// PruneVolumes removes unused volumes. If all is true, all unused volumes are removed, not just anonymous ones.
func PruneVolumes(all bool) error {
	args := []string{"volume", "prune", "-f"}
	if all {
		args = append(args, "-a")
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// PruneNetworks removes unused networks
func PruneNetworks() error {
	cmd := exec.Command("docker", "network", "prune", "-f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// ComposeExec runs a command in a service container
func ComposeExec(projectDir string, service string, cmdParts []string) error {
	args := []string{"compose", "exec", service}
	args = append(args, cmdParts...)

	cmd := exec.Command("docker", args...)
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// ComposeExecWithStdin runs a command in a service container with a custom stdin
func ComposeExecWithStdin(projectDir string, service string, cmdParts []string, stdin io.Reader) error {
	args := []string{"compose", "exec", "-T", service} // Use -T to disable pseudo-tty when piping
	args = append(args, cmdParts...)

	cmd := exec.Command("docker", args...)
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = stdin

	return cmd.Run()
}
