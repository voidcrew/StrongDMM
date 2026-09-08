package app

import (
	"os"
	"path/filepath"
	"sdmm/internal/ruin"

	"github.com/sqweek/dialog"
)

func (a *app) HasVoidcrewProject() bool {
	if !a.HasLoadedEnvironment() {
		return false
	}
	_, ok := a.loadedEnvironment.Objects["/datum/map_template/shuttle/voidcrew"]
	return ok
}

func (a *app) DoOpenProject() {
	startDir := ""
	if a.HasLoadedEnvironment() {
		startDir = a.loadedEnvironment.RootDir
	}
	path, err := dialog.File().Title("Open Voidcrew project").Filter("BYOND project", "dme").SetStartDir(startDir).Load()
	if err == nil {
		a.loadEnvironment(path)
	}
}

func (a *app) DoNewShip() {
	if a.HasVoidcrewProject() {
		a.layout.WsArea.OpenShip().BeginNewShip()
	}
}

func (a *app) HasRuinProject() bool {
	return a.HasLoadedEnvironment() && a.loadedEnvironment.Objects[ruin.Type] != nil
}

func (a *app) DoOpenRuinWorkspace() {
	if a.HasRuinProject() {
		a.layout.WsArea.OpenRuin()
	}
}

func (a *app) DoNewRuin() {
	if a.HasRuinProject() {
		a.layout.WsArea.OpenRuin().BeginNewRuin()
	}
}

func (a *app) HasSaveableWorkspace() bool {
	if a.HasActiveMap() {
		return true
	}
	ws := a.layout.WsArea.ActiveWorkspace()
	if ws == nil {
		return false
	}
	_, ok := ws.Content().(interface{ IsModified() bool })
	return ok
}

// Project directories may be dropped directly onto StrongDMM.exe.
func projectArgument(path string) string {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return filepath.Join(path, "tgstation.dme")
	}
	return path
}
