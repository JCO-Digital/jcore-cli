//go:build windows

package update

import "syscall"

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// detachSysProcAttr starts the background checker detached from this
// process's console, so it isn't killed when this process exits or the
// console window closes.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcess}
}
