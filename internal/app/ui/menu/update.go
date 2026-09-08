package menu

import (
	"sdmm/internal/app/ui/workshop"
	"sdmm/internal/app/window"
	"sdmm/internal/imguiext/icon"
	"sdmm/internal/imguiext/style"
	w "sdmm/internal/imguiext/widget"

	"github.com/SpaiR/imgui-go"
)

func (m *Menu) showUpdateMenu() {
	label := "Update available"
	switch m.updateStatus {
	case upStatusChecking:
		label = "Checking updates..."
	case upStatusUpdating:
		label = "Downloading update..."
	case upStatusUpdated:
		label = "Restart to update"
	case upStatusCurrent:
		label = "Up to date"
	case upStatusError:
		label = "Update needs attention"
	}
	w.Button(icon.SystemUpdate+" "+label+"##update_status", m.ShowUpdatePopup).
		Style(style.ButtonFrame{}).TextColor(style.Teal).Build()
	if m.updateOpen {
		imgui.OpenPopup("update_menu")
		m.updateOpen = false
	}
	workshop.PushStyle()
	if imgui.BeginPopup("update_menu") {
		width := 380 * window.PointSize()
		imgui.PushTextWrapPosV(width)
		if m.updateVersion != "" {
			imgui.TextColored(style.Amber, "StrongDMM "+m.updateVersion)
		}
		if m.updateDescription != "" {
			imgui.BeginChildV("release_notes", imgui.Vec2{X: width, Y: 125 * window.PointSize()}, false, 0)
			imgui.TextWrapped(m.updateDescription)
			imgui.EndChild()
			imgui.Separator()
		}
		if m.updateError != "" {
			imgui.TextWrapped(m.updateError)
			imgui.Separator()
		}
		switch m.updateStatus {
		case upStatusChecking:
			imgui.Text("Checking GitHub for updates...")
		case upStatusCurrent:
			imgui.Text("You're using the latest available version.")
			w.Button("Done", m.doHideUpdateButton).Build()
		case upStatusAvailable:
			if workshop.Button("Download update", true) {
				m.app.DoSelfUpdate()
			}
			w.Button("Skip this version", m.doIgnoreUpdate).Build()
		case upStatusUpdating:
			imgui.Text("Downloading and verifying the update...")
			imgui.Text("You can keep working.")
		case upStatusUpdated:
			imgui.Text("The update is ready. Save prompts appear before restarting.")
			if workshop.Button("Update & restart", true) {
				imgui.CloseCurrentPopup()
				m.app.DoRestart()
			}
			w.Button("Later", func() { imgui.CloseCurrentPopup() }).Build()
		case upStatusError:
			w.Button("Try again", m.app.DoCheckForUpdates).Build()
			imgui.SameLine()
			w.Button("Download manually", m.app.DoOpenUpdateDownload).Build()
		}
		imgui.PopTextWrapPos()
		imgui.EndPopup()
	}
	workshop.PopStyle()
}

func (m *Menu) ShowUpdatePopup() { m.updateOpen = true }
func (m *Menu) SetChecking() {
	m.updateStatus = upStatusChecking
	m.updateError, m.updateDescription, m.updateVersion = "", "", ""
}
func (m *Menu) SetUpToDate(version string) {
	m.updateStatus = upStatusCurrent
	m.updateVersion = version
	m.updateError, m.updateDescription = "", ""
}
func (m *Menu) SetRestartError(message string) {
	m.updateStatus = upStatusUpdated
	m.updateError = message
	m.ShowUpdatePopup()
}
func (m *Menu) doHideUpdateButton() { m.updateStatus = upStatusNone; imgui.CloseCurrentPopup() }
func (m *Menu) doIgnoreUpdate()     { m.doHideUpdateButton(); m.app.DoIgnoreUpdate() }
