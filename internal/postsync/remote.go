// Package postsync pushes a single local WordPress post to the project's
// configured remote (remoteHost/remotePath), uploading any media it
// references and rewriting local attachment IDs and *.localhost URLs.
//
// Everything runs inside the local "wordpress" container, exactly like
// `jcore pull`: that's where the jcore SSH key (~/.config/jcore/ssh, mounted
// as ~/.ssh), wp-cli and rsync live. The host never runs ssh itself.
package postsync

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/JCO-Digital/jcore/internal/docker"
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

const (
	containerService = "wordpress"
	localWebroot     = "/var/www/html"
)

// sshOptions are passed to every ssh invocation. BatchMode makes a missing
// key or passphrase fail instead of hanging on a prompt nobody can see
// (stdin is not a terminal here); accept-new trusts a host seen for the
// first time but still refuses one whose key has changed.
var sshOptions = []string{"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new"}

// DockerRunner is the real Runner, executing through `docker compose exec`.
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

func (d *DockerRunner) exec(parts []string, stdin string) (string, error) {
	var in io.Reader
	if stdin != "" {
		in = strings.NewReader(stdin)
	}
	stdout, stderr, err := docker.ComposeExecOutput(d.ProjectDir, containerService, parts, in)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		if msg == "" {
			return stdout, err
		}
		return stdout, fmt.Errorf("%w: %s", err, msg)
	}
	return stdout, nil
}

func (d *DockerRunner) Local(args ...string) (string, error) {
	return d.exec(append([]string{"wp"}, args...), "")
}

func (d *DockerRunner) remoteParts(args []string) []string {
	quoted := []string{"wp", "--path=" + shellQuote(d.Path)}
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	return append(append(d.sshCommand(), d.Host), strings.Join(quoted, " "))
}

func (d *DockerRunner) Remote(args ...string) (string, error) {
	return d.exec(d.remoteParts(args), "")
}

func (d *DockerRunner) RemoteStdin(stdin string, args ...string) (string, error) {
	if stdin == "" {
		// An empty stdin would leave wp-cli waiting; callers always send
		// content, but guard anyway with a single newline.
		stdin = "\n"
	}
	return d.exec(d.remoteParts(args), stdin)
}

func (d *DockerRunner) rsyncParts(rel string, extra ...string) []string {
	parts := []string{"rsync", "-a", "--relative", "-e", strings.Join(d.sshCommand(), " ")}
	parts = append(parts, extra...)
	return append(parts,
		localWebroot+"/wp-content/uploads/./"+rel,
		d.Host+":"+d.Path+"/wp-content/uploads/",
	)
}

func (d *DockerRunner) FileStatus(rel string) (FileState, error) {
	if _, err := d.exec([]string{"test", "-f", localWebroot + "/wp-content/uploads/" + rel}, ""); err != nil {
		return FileMissingLocal, nil
	}
	// A dry run with checksums itemizes what *would* be transferred:
	// "<f+++++++++" for a new file, "<f..." for one whose content differs,
	// and nothing (or a ".f" attribute-only line) for an identical one.
	out, err := d.exec(d.rsyncParts(rel, "--dry-run", "--checksum", "--itemize-changes"), "")
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
	_, err := d.exec(d.rsyncParts(rel, "--ignore-existing"), "")
	return err
}
