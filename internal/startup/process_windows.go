package startup

import (
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) {
	// Suppress a console without suppressing native editor or error windows.
	// STARTF_USESHOWWINDOW/SW_HIDE would override a GUI's first ShowWindow.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
