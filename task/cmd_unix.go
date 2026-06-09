//go:build !windows
// +build !windows

package task

import "os/exec"

func hideWindow(cmd *exec.Cmd) {
	// No-op on Unix
}
