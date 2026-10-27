// Package postsync pushes a single local WordPress post or menu to the
// project's configured remote (remoteHost/remotePath), uploading any media it
// references and rewriting attachment IDs and *.localhost URLs, or pulls one
// from the remote the same way.
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

// FileState is how an uploads-relative file on the site it's copied from
// compares to the same path on the site it's copied to (the remote when
// pushing, the local site when pulling).
type FileState int

const (
	// FileMissing: the target has no file at this path.
	FileMissing FileState = iota
	// FileIdentical: the target already has this exact file.
	FileIdentical
	// FileDiffers: the target has a *different* file at this path.
	FileDiffers
	// FileMissingSource: the site it's copied from doesn't have the file
	// either.
	FileMissingSource
)

func (s FileState) String() string {
	switch s {
	case FileMissing:
		return "missing"
	case FileIdentical:
		return "identical"
	case FileDiffers:
		return "DIFFERENT"
	case FileMissingSource:
		return "missing at source"
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
	// LocalStdin is Local with stdin fed to wp-cli.
	LocalStdin(stdin string, args ...string) (string, error)
	// FileStatus compares wp-content/uploads/<rel> locally and remotely,
	// for uploading, without changing anything.
	FileStatus(rel string) (FileState, error)
	// Upload copies wp-content/uploads/<rel> to the remote, never
	// overwriting a file that already exists there.
	Upload(rel string) error
	// PullFileStatus is FileStatus for downloading: it compares the remote
	// wp-content/uploads/<rel> to the local one.
	PullFileStatus(rel string) (FileState, error)
	// Download copies wp-content/uploads/<rel> from the remote, never
	// overwriting a file that already exists locally.
	Download(rel string) error
}

// localWPRoot is the WordPress root inside the wordpress container.
const localWPRoot = "/var/www/html"

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
func (d *DockerRunner) execContainer(parts []string, stdin string) (string, error) {
	wpout.LogCommand(parts)
	stdout, stderr, err := docker.ComposeExecOutput(d.ProjectDir, containerService, parts, stdinReader(stdin))
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
	return d.execContainer(append([]string{"wp"}, args...), "")
}

func (d *DockerRunner) LocalStdin(stdin string, args ...string) (string, error) {
	if stdin == "" {
		stdin = "\n" // see RemoteStdin
	}
	return d.execContainer(append([]string{"wp"}, args...), stdin)
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
		return FileMissingSource, nil
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

// parseItemize reads an upload's state from rsync --itemize-changes output.
func parseItemize(out, rel string) FileState {
	return itemizeState(out, rel, "<")
}

// itemizeState reads rel's state from rsync --itemize-changes output; dir
// is the transfer direction rsync prints ("<" sent, ">" received).
func itemizeState(out, rel, dir string) FileState {
	for _, line := range strings.Split(out, "\n") {
		flags, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || name != rel {
			continue
		}
		if strings.HasPrefix(flags, dir+"f+++") {
			return FileMissing
		}
		if strings.HasPrefix(flags, dir+"f") {
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

// remoteUploads is the remote uploads directory.
func (d *DockerRunner) remoteUploads() string {
	return d.Path + "/wp-content/uploads"
}

func (d *DockerRunner) pullRsyncParts(rel string, extra ...string) []string {
	parts := []string{"rsync", "-a", "--relative", "-e", strings.Join(d.sshCommand(), " ")}
	parts = append(parts, extra...)
	return append(parts,
		d.Host+":"+d.remoteUploads()+"/./"+rel,
		d.localUploads()+"/",
	)
}

func (d *DockerRunner) PullFileStatus(rel string) (FileState, error) {
	if rel == "" || strings.Contains(rel, "..") {
		return 0, errors.New("refusing to check suspicious path " + rel)
	}
	// rsync fails outright on a missing source, so check for it first.
	test := "if [ -f " + shellQuote(d.remoteUploads()+"/"+rel) + " ]; then echo yes; else echo no; fi"
	out, err := d.execHost(append(append(d.sshCommand(), d.Host), test), "")
	if err != nil {
		return 0, err
	}
	if lastLine(out) != "yes" {
		return FileMissingSource, nil
	}
	if err := requireRsync(); err != nil {
		return 0, err
	}
	out, err = d.execHost(d.pullRsyncParts(rel, "--dry-run", "--checksum", "--itemize-changes"), "")
	if err != nil {
		return 0, err
	}
	return itemizeState(out, rel, ">"), nil
}

func (d *DockerRunner) Download(rel string) error {
	if rel == "" || strings.Contains(rel, "..") {
		return errors.New("refusing to download suspicious path " + rel)
	}
	if err := requireRsync(); err != nil {
		return err
	}
	if err := os.MkdirAll(d.localUploads(), 0o755); err != nil {
		return err
	}
	_, err := d.execHost(d.pullRsyncParts(rel, "--ignore-existing"), "")
	return err
}

func requireRsync() error {
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("rsync not found on host: %w (install rsync to upload media)", err)
	}
	return nil
}
