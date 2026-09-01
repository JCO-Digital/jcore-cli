//go:build !windows

package update

import "syscall"

// detachSysProcAttr starts the background checker in its own session, so it
// isn't killed by a signal (e.g. Ctrl-C / SIGINT) sent to this process's
// terminal process group.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
