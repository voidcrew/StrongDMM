package app

import (
	"os"
	"path/filepath"
)

// Check program arguments to load dme/dmm files passed by.
func (a *app) checkProgramArgs() {
	// The first argument is always a path to the executable.
	if len(os.Args) < 2 {
		return
	}

	var envPath string
	var mapPaths []string
	var shipWorkspace bool

	for _, arg := range os.Args {
		if arg == "--ship-workspace" {
			shipWorkspace = true
		}
		switch filepath.Ext(arg) {
		case ".dme":
			envPath = arg
		case ".dmm":
			mapPaths = append(mapPaths, arg)
		}
	}

	if len(envPath) > 0 {
		if shipWorkspace {
			a.loadEnvironmentV(envPath, a.DoOpenShipWorkspace)
		} else {
			a.loadResource(envPath)
		}
	}

	for _, mapPath := range mapPaths {
		a.loadResource(mapPath)
	}
}
