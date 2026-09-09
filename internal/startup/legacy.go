package startup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// The last StrongDMM feed packages Voidworks under the filename required by old
// updaters. First launch creates the new executable; old shortcuts subsequently
// forward to it, so they follow later Voidworks updates and channel switches.
func legacyExecutable(executable string) (string, error) {
	destination := filepath.Join(filepath.Dir(executable), "Voidworks.exe")
	unlock, err := lockUpdate(destination)
	if err != nil {
		return "", err
	}
	defer unlock()
	if info, err := os.Lstat(destination); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("expected a regular executable: %s", destination)
		}
		return destination, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	source, err := os.Open(executable)
	if err != nil {
		return "", err
	}
	defer source.Close()
	file, err := os.CreateTemp(filepath.Dir(executable), ".voidworks-launch-*.exe")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	_, copyErr := io.Copy(file, source)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return "", err
	}
	if err := os.Rename(file.Name(), destination); err != nil {
		return "", err
	}
	return destination, nil
}
