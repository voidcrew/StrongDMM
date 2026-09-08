package app

import (
	"os"
	"path/filepath"
	"strings"
)

type startupArgs struct {
	project       string
	maps          []string
	shipWorkspace bool
	ruinWorkspace bool
}

func parseStartupArgs(args []string) startupArgs {
	var parsed startupArgs
	for _, arg := range args {
		if arg == "--ruin-workspace" {
			parsed.ruinWorkspace = true
			continue
		}
		if arg == "--ship-workspace" {
			parsed.shipWorkspace = true
			continue
		}
		path := projectArgument(arg)
		switch strings.ToLower(filepath.Ext(path)) {
		case ".dme":
			parsed.project = path
		case ".dmm":
			parsed.maps = append(parsed.maps, path)
		}
	}
	return parsed
}

func (a *app) checkProgramArgs() {
	args := parseStartupArgs(os.Args[1:])
	if args.project == "" && len(args.maps) > 0 {
		args.project, _ = findEnvironmentFileFromBase(args.maps[0])
	}
	if args.project != "" {
		a.loadEnvironmentV(args.project, func() {
			if args.shipWorkspace {
				a.DoOpenShipWorkspace()
			}
			if args.ruinWorkspace {
				a.DoOpenRuinWorkspace()
			}
			for _, path := range args.maps {
				a.loadMap(path, nil)
			}
		})
		return
	}
	for _, path := range args.maps {
		a.loadResource(path)
	}
}
