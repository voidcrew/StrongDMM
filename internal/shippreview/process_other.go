//go:build !windows

package shippreview

import (
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(*exec.Cmd) {}

func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	return err == nil && process.Signal(syscall.Signal(0)) == nil
}
