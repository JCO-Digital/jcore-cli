// Package proxy manages the shared Traefik reverse proxy that lets several
// JCore projects run at the same time. The proxy owns host ports 80/443 and
// routes each request by hostname (SNI for https) to the right project's
// own nginx; projects opt in through Docker labels, so the proxy itself
// holds no per-project state.
package proxy

import (
	_ "embed"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// ProjectName is the compose project name of the proxy stack.
	ProjectName = "jcore-proxy"
	// NetworkName is the shared Docker network the proxy and every
	// project's HTTP-facing services join.
	NetworkName = "jcore-proxy"
	// DashboardURL is where the Traefik dashboard is served.
	DashboardURL = "http://traefik.localhost"
)

// Ports are the host ports the proxy publishes.
var Ports = []int{80, 443}

//go:embed compose.yml
var composeFile []byte

// Dir returns the directory the proxy's compose file is written to.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jcore", "proxy"), nil
}

// writeComposeFile (re)writes the embedded compose file to Dir(), so the
// proxy always runs the configuration matching this jcore version.
func writeComposeFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "docker-compose.yml")
	return path, os.WriteFile(path, composeFile, 0644)
}

// compose runs a docker compose command against the proxy stack, streaming
// its output to the terminal.
func compose(args ...string) error {
	path, err := writeComposeFile()
	if err != nil {
		return err
	}
	cmd := exec.Command("docker", append([]string{"compose", "-f", path, "-p", ProjectName}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// EnsureNetwork creates the shared network if it doesn't exist yet (e.g. on
// first use, or after `docker network prune` removed it while unused).
func EnsureNetwork() error {
	if err := exec.Command("docker", "network", "inspect", NetworkName).Run(); err == nil {
		return nil
	}
	out, err := exec.Command("docker", "network", "create", "--label", "fi.jco.jcore.proxy=true", NetworkName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating network %s: %v: %s", NetworkName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Running reports whether the proxy container is up.
func Running() (bool, error) {
	out, err := exec.Command("docker", "ps", "-q",
		"--filter", "label=com.docker.compose.project="+ProjectName,
		"--filter", "status=running").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// PortHolder is something other than the proxy occupying one of its host
// ports.
type PortHolder struct {
	Port int
	// Container is the holding container's name, or "" for a process
	// running directly on the host.
	Container string
	// Project is the holding container's compose project, if any.
	Project string
	// Path is the compose project's working directory, if known.
	Path string
}

func (h PortHolder) String() string {
	switch {
	case h.Project != "" && h.Path != "":
		return fmt.Sprintf("port %d: compose project %q (%s)", h.Port, h.Project, h.Path)
	case h.Container != "":
		return fmt.Sprintf("port %d: container %q", h.Port, h.Container)
	default:
		return fmt.Sprintf("port %d: a process on the host (not a Docker container)", h.Port)
	}
}

// PortConflictError is returned by Ensure when the proxy can't start
// because something else holds its ports.
type PortConflictError struct {
	Holders []PortHolder
}

func (e *PortConflictError) Error() string {
	lines := []string{"the JCore proxy can't start because its ports are taken:"}
	for _, h := range e.Holders {
		lines = append(lines, "  "+h.String())
	}
	return strings.Join(lines, "\n")
}

// Projects returns the distinct compose projects among the holders, i.e.
// the ones that can be stopped with StopComposeProject.
func (e *PortConflictError) Projects() []string {
	var projects []string
	seen := map[string]bool{}
	for _, h := range e.Holders {
		if h.Project != "" && !seen[h.Project] {
			seen[h.Project] = true
			projects = append(projects, h.Project)
		}
	}
	return projects
}

// PortHolders returns whatever, other than the proxy, currently holds the
// proxy's host ports: containers publishing them (typically a project
// started by an older jcore that still binds 80/443 itself), or failing
// that, a host process listening on them.
func PortHolders() ([]PortHolder, error) {
	var holders []PortHolder
	for _, port := range Ports {
		out, err := exec.Command("docker", "ps",
			"--filter", fmt.Sprintf("publish=%d", port),
			"--format", `{{.Names}}|{{.Label "com.docker.compose.project"}}|{{.Label "com.docker.compose.project.working_dir"}}`).Output()
		if err != nil {
			return nil, err
		}

		found := false
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			parts := strings.SplitN(line, "|", 3)
			if len(parts) != 3 || parts[0] == "" {
				continue
			}
			found = true
			if parts[1] == ProjectName {
				continue
			}
			holders = append(holders, PortHolder{Port: port, Container: parts[0], Project: parts[1], Path: parts[2]})
		}

		if !found && listening(port) {
			holders = append(holders, PortHolder{Port: port})
		}
	}
	return holders, nil
}

// listening reports whether something accepts connections on localhost:port.
func listening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Ensure makes sure the proxy is running: it creates the shared network if
// needed and starts the proxy unless it's already up. If something else
// holds the proxy's ports, it returns a *PortConflictError without starting
// anything, so the caller can decide what to do about it.
func Ensure() error {
	if err := EnsureNetwork(); err != nil {
		return err
	}

	running, err := Running()
	if err != nil {
		return err
	}
	if running {
		return nil
	}

	holders, err := PortHolders()
	if err != nil {
		return err
	}
	if len(holders) > 0 {
		return &PortConflictError{Holders: holders}
	}

	fmt.Println("Starting the JCore proxy...")
	return compose("up", "-d", "--remove-orphans")
}

// Stop takes the proxy down. Projects keep running but become unreachable
// from the host until the proxy is started again.
func Stop() error {
	return compose("down")
}

// Logs follows the proxy's logs.
func Logs() error {
	return compose("logs", "-f", "--since", "5m")
}

// StopComposeProject stops every container of the named compose project.
// Used to clear a port conflict with a project started by an older jcore.
func StopComposeProject(name string) error {
	cmd := exec.Command("docker", "compose", "-p", name, "stop")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
