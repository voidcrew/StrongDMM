package app

import (
	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/dialog"
	"sdmm/internal/app/ui/workshop"
	w "sdmm/internal/imguiext/widget"
	"sdmm/internal/shippreview"
)

func (a *app) ShipFilesSaved() {
	dme := a.LoadedEnvironment()
	if dme != nil {
		a.previews.Request(dme.RootDir, dme.RootFile)
	}
}

func (a *app) MapFileSaved(path string) {
	if dme := a.LoadedEnvironment(); dme != nil && shippreview.IsShipMap(dme.RootDir, path) {
		a.ShipFilesSaved()
	}
}

func (a *app) ShipPreviewStatus() shippreview.Status {
	if dme := a.LoadedEnvironment(); dme != nil {
		return a.previews.Status(dme.RootDir)
	}
	return shippreview.Status{}
}

func (a *app) DoShipPreviews() {
	dialog.Open(dialog.TypeCustom{Title: "Ship purchase previews", CloseButton: true, Layout: w.Layout{
		w.Custom(func() {
			imgui.TextWrapped("Ship saves automatically regenerate purchase previews in the background. You can keep editing or close the editor while they finish.")
			workshop.Gap()
			status := a.ShipPreviewStatus()
			workshop.PreviewStatus(status, a.ShipFilesSaved)
			if status.Phase == "" {
				imgui.TextWrapped("No preview generation has run for this project yet.")
			}
			if status.Phase == "" || status.Phase == "complete" {
				if imgui.Button("Regenerate previews") {
					a.ShipFilesSaved()
				}
			}
			workshop.Gap()
			imgui.TextWrapped("Requires Python 3.10 or newer, Pillow, and dmm-tools. Install Pillow with: python -m pip install Pillow. Put dmm-tools on PATH or set DMM_TOOLS to its executable, then restart the editor.")
		}),
	}})
}
