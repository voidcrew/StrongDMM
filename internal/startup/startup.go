package startup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"sdmm/internal/env"

	"github.com/sqweek/dialog"
)

const editorArgument = "--strongdmm-editor"

// Run keeps crash reporting independent of the graphics context and editor
// goroutines. On Windows the same executable quietly supervises its editor
// process, so launching StrongDMM.exe needs no console or external launcher.
func Run(start func()) int {
	if len(os.Args) > 1 && os.Args[1] == editorArgument {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		start()
		if updateScheduled {
			return updateExitCode
		}
		return 0
	}
	if runtime.GOOS != "windows" {
		start()
		return 0
	}
	file, err := sessionLog()
	if err != nil {
		showError("Unable to create a startup log.\n\n" + err.Error())
		return 1
	}
	fmt.Fprintf(file, "%s %s\nStarted: %s\nSystem: %s/%s\n", env.Title, env.Version, time.Now().Format(time.RFC3339), runtime.GOOS, runtime.GOARCH)
	executable, err := os.Executable()
	code := 1
	if err == nil {
		cleanupUpdate(executable)
		requestDir, requestErr := os.MkdirTemp("", "StrongDMM-restart-")
		if requestErr != nil {
			err = requestErr
		} else {
			defer os.RemoveAll(requestDir)
			requestPath := filepath.Join(requestDir, "update.json")
			args := append([]string{editorArgument}, os.Args[1:]...)
			code, err = runEditor(executable, args, file, requestEnvironment+"="+requestPath)
			if code == updateExitCode {
				err = applyUpdateRequest(requestPath, executable, file)
				if err == nil {
					code = 0
				}
			}
		}
	}
	fmt.Fprintf(file, "\nExit code: %d (0x%08X)\n", code, uint32(code))
	if err != nil {
		fmt.Fprintln(file, err)
	}
	_ = file.Close()
	if code != 0 {
		showError(fmt.Sprintf("StrongDMM stopped unexpectedly.\n\nExit code: %d (0x%08X)\n\nDetails were saved to:\n%s", code, uint32(code), file.Name()))
	}
	return code
}

func runEditor(executable string, args []string, file *os.File, extraEnv ...string) (int, error) {
	cmd := exec.Command(executable, args...)
	cmd.Env = updateEnvironment(extraEnv...)
	configureProcess(cmd)
	// Native file handles avoid pipe buffering and retain output even if the
	// editor aborts from another goroutine or inside a native dependency.
	cmd.Stdout, cmd.Stderr = file, file
	err := cmd.Start()
	if err == nil {
		fmt.Fprintf(file, "Editor process: %d\n", cmd.Process.Pid)
		err = cmd.Wait()
	}
	if err == nil {
		return 0, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode(), err
	}
	return 1, err
}

func sessionLog() (*os.File, error) {
	var folders []string
	if profile, err := env.ProfileDir(); err == nil {
		folders = append(folders, filepath.Join(profile, "logs"))
	}
	folders = append(folders, filepath.Join(os.TempDir(), "StrongDMM-Voidcrew", "logs"))
	var failures []error
	for _, folder := range folders {
		if err := os.MkdirAll(folder, 0700); err != nil {
			failures = append(failures, err)
			continue
		}
		file, err := os.CreateTemp(folder, "session-"+time.Now().Format("20060102-150405")+"-*.log")
		if err == nil {
			return file, nil
		}
		failures = append(failures, err)
	}
	return nil, errors.Join(failures...)
}

func showError(message string) {
	dialog.Message("%s", message).Title(env.Title).Error()
}
