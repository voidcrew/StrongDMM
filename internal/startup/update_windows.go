package startup

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/windows"
)

func lockUpdate(executable string) (func(), error) {
	folder, err := filepath.EvalSymlinks(filepath.Dir(executable))
	if err != nil {
		return nil, err
	}
	executable = filepath.Join(folder, filepath.Base(executable))
	name := fmt.Sprintf("Local\\Voidworks-Update-%x", sha256.Sum256([]byte(strings.ToLower(executable))))
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return nil, err
	}
	runtime.LockOSThread() // Windows mutex ownership belongs to an OS thread.
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || (status != windows.WAIT_OBJECT_0 && status != windows.WAIT_ABANDONED) {
		windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("another Voidworks instance is applying an update; try again after it finishes")
	}
	return func() { windows.ReleaseMutex(handle); windows.CloseHandle(handle); runtime.UnlockOSThread() }, nil
}
