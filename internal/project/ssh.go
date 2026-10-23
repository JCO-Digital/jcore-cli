package project

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
)

// BuildSSHArgs constructs the ssh arguments for connecting to remoteHost. With no
// command it opens an interactive login shell, starting in remotePath when set.
// Otherwise command is run on the remote, from remotePath when set.
func BuildSSHArgs(remoteHost, remotePath string, command []string) ([]string, error) {
	host, port := ParseSSHHostPort(remoteHost)
	if host == "" {
		return nil, errors.New("remoteHost is empty")
	}

	var args []string
	if len(command) == 0 {
		args = append(args, "-t")
	}
	if port != "" {
		args = append(args, "-p", port)
	}
	args = append(args, host)

	remotePath = strings.TrimRight(remotePath, "/")
	switch {
	case len(command) == 0 && remotePath != "":
		args = append(args, "cd "+sshQuote(remotePath)+" 2>/dev/null; exec \"$SHELL\" -l")
	case len(command) > 0 && remotePath != "":
		args = append(args, "cd "+sshQuote(remotePath)+" && "+strings.Join(command, " "))
	case len(command) > 0:
		args = append(args, command...)
	}

	return args, nil
}

// RunSSH knocks the remote host if needed and connects to it over ssh with the
// current terminal attached. The returned error is an *exec.ExitError when ssh
// (or the remote command) exits non-zero.
func RunSSH(command []string) error {
	remoteHost := viper.GetString("remoteHost")
	if remoteHost == "" {
		return errors.New("remoteHost is not configured in jcore.toml")
	}

	args, err := BuildSSHArgs(remoteHost, viper.GetString("remotePath"), command)
	if err != nil {
		return err
	}

	KnockIfNeeded()

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// sshQuote quotes s for the POSIX login shell ssh hands the command string to.
func sshQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
