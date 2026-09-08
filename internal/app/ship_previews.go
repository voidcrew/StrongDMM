package app

import (
	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/dialog"
	"sdmm/internal/app/ui/workshop"
	"sdmm/internal/app/window"
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
			imgui.Dummy(imgui.Vec2{X: 420 * window.PointSize()})
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
			if shippreview.Bundled() {
				imgui.TextWrapped("Preview tools are included. No additional installation is needed.")
			} else {
				imgui.TextWrapped("This source build requires Python 3.10 or newer, Pillow, and dmm-tools. The Windows release includes these tools.")
			}
		}),
	}})
}
