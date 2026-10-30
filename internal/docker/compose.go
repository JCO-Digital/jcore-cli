package docker

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/JCO-Digital/jcore/internal/wpout"
	"golang.org/x/term"
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

	return WithCredentialHint(cmd.Run())
}

// ComposePull runs docker compose pull
func ComposePull(projectDir string) error {
	cmd := exec.Command("docker", "compose", "pull")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return WithCredentialHint(cmd.Run())
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

// ComposeDown stops and removes the project's containers, its own networks
// and its named volumes (docker compose down -v --remove-orphans).
func ComposeDown(projectDir string) error {
	cmd := exec.Command("docker", "compose", "down", "--volumes", "--remove-orphans")
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

// ExecOptions configures ComposeRun.
type ExecOptions struct {
	// TTY allocates a pseudo-terminal. A terminal merges the command's
	// stderr into its stdout, and docker refuses one when jcore's stdin
	// isn't a terminal, so only set it when stdin, stdout and stderr all
	// are (see StdioIsTerminal).
	TTY bool
	// Env holds NAME=value pairs set for the command only.
	Env []string
}

// StdioIsTerminal reports whether jcore's stdin, stdout and stderr are all
// terminals, i.e. it was started interactively rather than from a script,
// a pipe or an agent.
func StdioIsTerminal() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
		if !term.IsTerminal(int(f.Fd())) {
			return false
		}
	}
	return true
}

// composeExecArgs builds the docker arguments of ComposeRun.
func composeExecArgs(service string, opts ExecOptions, cmdParts []string) []string {
	args := []string{"compose", "exec"}
	if !opts.TTY {
		args = append(args, "-T")
	}
	for _, env := range opts.Env {
		args = append(args, "-e", env)
	}
	args = append(args, service)
	return append(args, cmdParts...)
}

// ComposeRun runs a command in a service container wired straight to
// jcore's stdin, stdout and stderr. Without a TTY, stdout and stderr stay
// separate streams. The returned error is an *exec.ExitError when the
// command (or docker itself) exits non-zero, carrying its exit code.
//
// Ctrl-C is left to the command: jcore keeps running until it exits, so
// its exit code and last output still come through.
func ComposeRun(projectDir string, service string, opts ExecOptions, cmdParts []string) error {
	cmd := exec.Command("docker", composeExecArgs(service, opts, cmdParts)...)
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	return cmd.Run()
}

// ComposeExecFiltered runs a non-interactive command (wp-cli, or a script
// calling it) in a service container with its stderr passed through
// wpout's warning filter, and its full output written to the command log.
// It runs with -T so stdout and stderr stay separate streams; when jcore's
// own stdout is a terminal, SHELL_PIPE=0 tells wp-cli to keep its
// terminal formatting (tables, colours) despite the pipe.
func ComposeExecFiltered(projectDir string, service string, cmdParts []string) error {
	args := []string{"compose", "exec", "-T"}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		args = append(args, "-e", "SHELL_PIPE=0")
	}
	args = append(args, service)
	args = append(args, cmdParts...)

	wpout.LogCommand(cmdParts)
	stderr := wpout.Stderr()
	defer stderr.Close()

	cmd := exec.Command("docker", args...)
	cmd.Dir = projectDir
	cmd.Stdout = io.MultiWriter(os.Stdout, wpout.Log())
	cmd.Stderr = stderr

	return cmd.Run()
}

// ComposeExecCaptured runs a command in a service container and returns its
// combined stdout+stderr instead of streaming it to the terminal, for a
// caller that wants to inspect or suppress the output itself (e.g. a silent
// polling loop probing readiness, which shouldn't spam the terminal with
// "container not ready yet" errors on every retry). Runs non-interactively
// (-T), since it isn't wired to a live terminal on either side.
func ComposeExecCaptured(projectDir string, service string, cmdParts []string) (string, error) {
	args := []string{"compose", "exec", "-T", service}
	args = append(args, cmdParts...)

	cmd := exec.Command("docker", args...)
	cmd.Dir = projectDir

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ComposeExecCapturedTimeout is ComposeExecCaptured with an upper bound on
// how long the command may run. A command still running when timeout
// elapses is killed and context.DeadlineExceeded is returned. Use it for
// polling containers that may not be healthy yet, where a single hung
// `docker compose exec` would otherwise block the caller indefinitely.
func ComposeExecCapturedTimeout(projectDir string, service string, cmdParts []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"compose", "exec", "-T", service}
	args = append(args, cmdParts...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = projectDir

	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), ctx.Err()
	}
	return string(out), err
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

// ComposeExecOutput runs a command in a service container non-interactively
// (-T) and returns its stdout and stderr separately, optionally feeding it
// stdin (nil for none). Keeping stdout apart matters for callers parsing
// machine-readable output (e.g. wp-cli --format=json), which PHP notices on
// stderr would otherwise corrupt.
func ComposeExecOutput(projectDir string, service string, cmdParts []string, stdin io.Reader) (string, string, error) {
	args := []string{"compose", "exec", "-T", service}
	args = append(args, cmdParts...)

	cmd := exec.Command("docker", args...)
	cmd.Dir = projectDir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = stdin

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
