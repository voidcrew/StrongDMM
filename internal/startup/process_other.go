//go:build !windows

package startup

import "os/exec"

func configureProcess(*exec.Cmd) {}
