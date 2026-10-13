// Package postsync pushes a single local WordPress post to the project's
// configured remote (remoteHost/remotePath), uploading any media it
// references and rewriting local attachment IDs and *.localhost URLs.
//
// Local wp-cli runs inside the "wordpress" container, while ssh and rsync to
// the remote run on the host, exactly like `jcore pull`: they use the host's
// ssh config and agent, so a container started before the agent last
// restarted can't break them with a stale socket mount.
package postsync

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JCO-Digital/jcore/internal/docker"
	"github.com/JCO-Digital/jcore/internal/wpout"
)

// FileState is how an uploads-relative file on the local site compares to
// the same path on the remote.
type FileState int

const (
	// FileMissing: the remote has no file at this path.
	FileMissing FileState = iota
	// FileIdentical: the remote already has this exact file.
	FileIdentical
	// FileDiffers: the remote has a *different* file at this path.
	FileDiffers
	// FileMissingLocal: the local site doesn't have the file either.
	FileMissingLocal
)

func (s FileState) String() string {
	switch s {
	case FileMissing:
		return "upload"
	case FileIdentical:
		return "exists"
	case FileDiffers:
		return "DIFFERENT on remote"
	case FileMissingLocal:
		return "missing locally"
	}
	return "unknown"
}

// Runner executes wp-cli locally and on the remote, and moves upload files
// between them. It's an interface so the sync flow can be tested without
// Docker or SSH.
type Runner interface {
	// Local runs `wp <args>` against the local site and returns stdout.
	Local(args ...string) (string, error)
	// Remote runs `wp <args>` against the remote site and returns stdout.
	Remote(args ...string) (string, error)
	// RemoteStdin is Remote with stdin fed to wp-cli.
	RemoteStdin(stdin string, args ...string) (string, error)
	// FileStatus compares wp-content/uploads/<rel> locally and remotely
	// without changing anything.
	FileStatus(rel string) (FileState, error)
	// Upload copies wp-content/uploads/<rel> to the remote, never
	// overwriting a file that already exists there.
	Upload(rel string) error
}

const containerService = "wordpress"

// sshOptions are passed to every ssh invocation. BatchMode makes a missing
// key or passphrase fail instead of hanging on a prompt nobody can see
// (stdin is fed by us, not the terminal); accept-new trusts a host seen for
// the first time but still refuses one whose key has changed.
var sshOptions = []string{"-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=10"}

// DockerRunner is the real Runner: local wp-cli goes through
// `docker compose exec`, remote wp-cli and rsync run on the host.
type DockerRunner struct {
	ProjectDir string
	Host       string // [user@]host, without port
	Port       string // "" for the ssh default
	Path       string // WordPress root on the remote
}

// NewDockerRunner builds a DockerRunner from the remoteHost/remotePath
// settings (remoteHost may carry a ":port" suffix, as `wp --ssh` accepts).
func NewDockerRunner(projectDir, remoteHost, remotePath string) *DockerRunner {
	host, port := SplitSSHHost(remoteHost)
	return &DockerRunner{ProjectDir: projectDir, Host: host, Port: port, Path: strings.TrimRight(remotePath, "/")}
}

// SplitSSHHost splits "user@host:port" into "user@host" and "port". A value
// without a numeric port suffix is returned unchanged with an empty port.
func SplitSSHHost(remoteHost string) (string, string) {
	h := strings.TrimSpace(remoteHost)
	if strings.Contains(h, "]:") { // [ipv6]:port
		i := strings.LastIndex(h, "]:")
		if isDigits(h[i+2:]) {
			return h[:i+1], h[i+2:]
		}
		return h, ""
	}
	if strings.Count(h, ":") == 1 {
		i := strings.LastIndex(h, ":")
		if isDigits(h[i+1:]) {
			return h[:i], h[i+1:]
		}
	}
	return h, ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// shellQuote quotes s for a POSIX shell (the remote login shell that ssh
// hands the command string to).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (d *DockerRunner) sshCommand() []string {
	cmd := append([]string{"ssh"}, sshOptions...)
	if d.Port != "" {
		cmd = append(cmd, "-p", d.Port)
	}
	return cmd
}

func stdinReader(stdin string) io.Reader {
	if stdin == "" {
		return nil
	}
	return strings.NewReader(stdin)
}

// execContainer runs parts inside the wordpress container.
func (d *DockerRunner) execContainer(parts []string) (string, error) {
	wpout.LogCommand(parts)
	stdout, stderr, err := docker.ComposeExecOutput(d.ProjectDir, containerService, parts, nil)
	return finishExec(stdout, stderr, err)
}

// execHost runs parts on the host, in the project directory.
func (d *DockerRunner) execHost(parts []string, stdin string) (string, error) {
	wpout.LogCommand(parts)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = d.ProjectDir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = stdinReader(stdin)
	err := cmd.Run()
	return finishExec(stdout.String(), stderr.String(), err)
}

func finishExec(stdout, stderr string, err error) (string, error) {
	if err != nil {
		_, _ = wpout.Log().Write([]byte(stderr))
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		if msg == "" {
			return stdout, err
		}
		return stdout, fmt.Errorf("%w: %s", err, msg)
	}
	// stdout is data for the caller; stderr warnings go through the filter.
	if stderr != "" {
		w := wpout.Stderr()
		_, _ = io.WriteString(w, stderr)
		_ = w.Close()
	}
	return stdout, nil
}

func (d *DockerRunner) Local(args ...string) (string, error) {
	return d.execContainer(append([]string{"wp"}, args...))
}

func (d *DockerRunner) remoteParts(args []string) []string {
	quoted := []string{"wp", "--path=" + shellQuote(d.Path)}
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	return append(append(d.sshCommand(), d.Host), strings.Join(quoted, " "))
}

func (d *DockerRunner) Remote(args ...string) (string, error) {
	return d.execHost(d.remoteParts(args), "")
}

func (d *DockerRunner) RemoteStdin(stdin string, args ...string) (string, error) {
	if stdin == "" {
		// An empty stdin would leave wp-cli waiting; callers always send
		// content, but guard anyway with a single newline.
		stdin = "\n"
	}
	return d.execHost(d.remoteParts(args), stdin)
}

func (d *DockerRunner) localUploads() string {
	return filepath.Join(d.ProjectDir, "wp-content", "uploads")
}

func (d *DockerRunner) rsyncParts(rel string, extra ...string) []string {
	parts := []string{"rsync", "-a", "--relative", "-e", strings.Join(d.sshCommand(), " ")}
	parts = append(parts, extra...)
	return append(parts,
		d.localUploads()+"/./"+rel,
		d.Host+":"+d.Path+"/wp-content/uploads/",
	)
}

func (d *DockerRunner) FileStatus(rel string) (FileState, error) {
	if info, err := os.Stat(filepath.Join(d.localUploads(), rel)); err != nil || !info.Mode().IsRegular() {
		return FileMissingLocal, nil
	}
	if err := requireRsync(); err != nil {
		return 0, err
	}
	// A dry run with checksums itemizes what *would* be transferred:
	// "<f+++++++++" for a new file, "<f..." for one whose content differs,
	// and nothing (or a ".f" attribute-only line) for an identical one.
	out, err := d.execHost(d.rsyncParts(rel, "--dry-run", "--checksum", "--itemize-changes"), "")
	if err != nil {
		return 0, err
	}
	return parseItemize(out, rel), nil
}

func parseItemize(out, rel string) FileState {
	for _, line := range strings.Split(out, "\n") {
		flags, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || name != rel {
			continue
		}
		if strings.HasPrefix(flags, "<f+++") {
			return FileMissing
		}
		if strings.HasPrefix(flags, "<f") {
			return FileDiffers
		}
	}
	return FileIdentical
}

func (d *DockerRunner) Upload(rel string) error {
	if rel == "" || strings.Contains(rel, "..") {
		return errors.New("refusing to upload suspicious path " + rel)
	}
	if err := requireRsync(); err != nil {
		return err
	}
	_, err := d.execHost(d.rsyncParts(rel, "--ignore-existing"), "")
	return err
}

func requireRsync() error {
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("rsync not found on host: %w (install rsync to upload media)", err)
	}
	return nil
}
