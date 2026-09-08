package workshop

import (
	"github.com/SpaiR/imgui-go"
	"github.com/skratchdot/open-golang/open"
	"os"
	"sdmm/internal/imguiext/style"
	"sdmm/internal/shippreview"
)

func PreviewStatus(status shippreview.Status, retry func()) {
	if status.Phase == "" {
		return
	}
	Section("PURCHASE PREVIEWS", style.Teal)
	imgui.TextWrapped(status.Message)
	if status.Phase == "failed" || status.Phase == "error" {
		if imgui.Button("Retry previews") {
			retry()
		}
		Muted("Help > Ship Purchase Previews")
	}
	_, logErr := os.Stat(status.Log)
	if status.Log != "" && logErr == nil && imgui.Button("Open preview log") {
		_ = open.Run(status.Log)
	}
}
