package util

import (
	"path/filepath"
	"runtime"
	"strings"
)

// SamePath compares normalized paths using the platform's usual casing rules.
func SamePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
