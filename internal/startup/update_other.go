//go:build !windows

package startup

import "fmt"

func lockUpdate(string) (func(), error) {
	return nil, fmt.Errorf("automatic updates require Windows x64")
}
