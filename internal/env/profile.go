package env

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ProfileDir is shared by the editor and its native startup reporter.
func ProfileDir() (string, error) {
	if runtime.GOOS == "windows" {
		root, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return migrateProfile(root, "Voidworks", "StrongDMM-Voidcrew")
	}
	root, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return migrateProfile(root, ".voidworks", ".strongdmm-voidcrew")
}

// Copy into a sibling temporary directory before publishing the new profile.
// Keep the original intact for older installations, and never overwrite a
// Voidworks profile that already exists, including on concurrent first starts.
func migrateProfile(root, name, legacyName string) (string, error) {
	destination := filepath.Join(root, name)
	if info, err := os.Stat(destination); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("profile is not a directory: %s", destination)
		}
		return destination, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	legacy := filepath.Join(root, legacyName)
	if _, err := os.Stat(legacy); os.IsNotExist(err) {
		return destination, nil
	} else if err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(root, ".voidworks-profile-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	if err := os.CopyFS(staging, os.DirFS(legacy)); err != nil {
		return "", fmt.Errorf("copy existing editor profile: %w", err)
	}
	if err := os.Rename(staging, destination); err != nil {
		if info, statErr := os.Stat(destination); statErr != nil || !info.IsDir() {
			return "", fmt.Errorf("finish profile migration: %w", err)
		}
	}
	return destination, nil
}
