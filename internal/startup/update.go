package startup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sdmm/internal/app/selfupdate"
	"sdmm/internal/env"
)

const updateExitCode = 85
const requestEnvironment = "VOIDWORKS_UPDATE_REQUEST"
const cleanupEnvironment = "VOIDWORKS_UPDATE_CLEANUP"

var updateScheduled bool

type updateRequest struct {
	Staged selfupdate.Staged
	Args   []string
}

func CanRestartForUpdate() bool { return selfupdate.Supported() && os.Getenv(requestEnvironment) != "" }

// ScheduleUpdate is called only after the normal close/save checks succeeded.
// The child then disposes the graphics context and saves its profile normally.
func ScheduleUpdate(staged selfupdate.Staged, args []string) error {
	if !CanRestartForUpdate() {
		return fmt.Errorf("restart Voidworks.exe normally before applying updates")
	}
	data, err := json.Marshal(updateRequest{Staged: staged, Args: args})
	if err != nil {
		return err
	}
	path := os.Getenv(requestEnvironment)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("prepare update restart: %w", err)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(path)
		return err
	}
	updateScheduled = true
	return nil
}

func updateEnvironment(extra ...string) []string {
	var result []string
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if !strings.EqualFold(name, requestEnvironment) && !strings.EqualFold(name, cleanupEnvironment) &&
			!strings.EqualFold(name, "STRONGDMM_UPDATE_REQUEST") && !strings.EqualFold(name, "STRONGDMM_UPDATE_CLEANUP") {
			result = append(result, value)
		}
	}
	return append(result, extra...)
}

func restartProcess(executable string, args []string, cleanup string) error {
	cmd := exec.Command(executable, args...)
	configureProcess(cmd)
	cmd.Env = updateEnvironment()
	if cleanup != "" {
		cmd.Env = append(cmd.Env, cleanupEnvironment+"="+cleanup)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}

func applyUpdateRequest(path, executable string, file *os.File) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return fmt.Errorf("update restart request is too large")
	}
	var request updateRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return err
	}
	err = installAndRestart(request, executable, file)
	if err != nil {
		// Report the specific error before reopening the previous installation.
		fmt.Fprintln(file, "Update failed:", err)
		showError("Unable to apply the update.\n\n" + err.Error() + "\n\nVoidworks will try to reopen the previous build.")
		return restartProcess(executable, request.Args, "")
	}
	return nil
}

func installAndRestart(request updateRequest, executable string, file *os.File) error {
	if !selfupdate.CanInstall(request.Staged.Release, env.Version) {
		return fmt.Errorf("update is not newer in this channel or an explicitly requested channel switch")
	}
	unlock, err := lockUpdate(executable)
	if err != nil {
		return err
	}
	defer unlock()
	fmt.Fprintf(file, "Installing update %s\n", request.Staged.Release.Version)
	change, err := selfupdate.Install(request.Staged, executable)
	if err == nil {
		err = restartProcess(executable, request.Args, request.Staged.Directory)
		if err != nil {
			err = errors.Join(err, change.Rollback())
		}
	}
	return err
}

func cleanupDirectory() string {
	if folder := os.Getenv(cleanupEnvironment); folder != "" {
		return folder
	}
	return os.Getenv("STRONGDMM_UPDATE_CLEANUP")
}

func cleanupUpdate(executable string) {
	folder := cleanupDirectory()
	if folder == "" {
		return
	}
	// The old supervisor can still have its executable open for a moment. The
	// discard method restricts deletion to a staging directory beside this exe.
	go func() {
		staged := selfupdate.Staged{Directory: filepath.Clean(folder)}
		for attempt := 0; attempt < 30; attempt++ {
			if err := staged.Discard(executable); err == nil || os.IsNotExist(err) {
				return
			}
			time.Sleep(time.Second)
		}
	}()
}
