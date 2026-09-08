package env

import (
	"os"
	"path/filepath"
	"runtime"
)

// ProfileDir is shared by the editor and its native startup reporter.
func ProfileDir() (string, error) {
	if runtime.GOOS == "windows" {
		root, err := os.UserConfigDir()
		return filepath.Join(root, "StrongDMM-Voidcrew"), err
	}
	root, err := os.UserHomeDir()
	return filepath.Join(root, ".strongdmm-voidcrew"), err
}
