package cpwsarea

import (
	"testing"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/command"
	"sdmm/internal/app/ui/cpwsarea/workspace"
)

type focusTestApp struct {
	App
	commands *command.Storage
}

func (a *focusTestApp) CommandStorage() *command.Storage { return a.commands }
func (*focusTestApp) IsLayoutReset() bool                { return false }
func (*focusTestApp) OnWorkspaceSwitched()               {}

type focusTestContent struct {
	workspace.Content
	name    string
	frames  int
	process func()
}

func (c *focusTestContent) Name() string  { return c.name }
func (c *focusTestContent) Title() string { return c.name }
func (c *focusTestContent) Process() {
	c.frames++
	if c.process != nil {
		c.process()
	}
}

func TestWorkspaceFocusDoesNotCancelFirstToolbarClick(t *testing.T) {
	context := imgui.CreateContext(nil)
	defer context.Destroy()
	io := imgui.CurrentIO()
	io.SetIniFilename("")
	io.SetConfigFlags(imgui.ConfigFlagsDockingEnable)
	io.SetDisplaySize(imgui.Vec2{X: 800, Y: 600})
	io.SetDeltaTime(1.0 / 60)
	io.Fonts().TextureDataRGBA32()
	area := &WsArea{app: &focusTestApp{commands: command.NewStorage()}}
	frame := func() {
		imgui.NewFrame()
		dockID := imgui.DockSpaceOverViewportV(imgui.MainViewport(), imgui.DockNodeFlagsNone)
		area.Process(int32(dockID))
		imgui.Render()
	}
	mapped := &focusTestContent{name: "Map"}
	mapWS := workspace.New(mapped)
	area.addWorkspace(mapWS)
	mapWS.SetTriggerFocus(true)
	for i := 0; i < 4; i++ {
		frame()
	}
	// A workspace receives focus automatically when it first appears. Its explicit
	// request must not remain armed and cancel a later click in another window.
	var button imgui.Vec2
	clicks := 0
	mapped.process = func() {
		imgui.SetNextWindowPos(imgui.Vec2{X: 40, Y: 80})
		imgui.SetNextWindowSize(imgui.Vec2{X: 200, Y: 80})
		imgui.BeginV("Toolbar", nil, imgui.WindowFlagsNoFocusOnAppearing|imgui.WindowFlagsNoDocking)
		if imgui.Button("Preview") {
			clicks++
		}
		lo, hi := imgui.ItemRectMin(), imgui.ItemRectMax()
		button = imgui.Vec2{X: (lo.X + hi.X) / 2, Y: (lo.Y + hi.Y) / 2}
		imgui.End()
	}
	for i := 0; i < 3; i++ {
		frame()
	}
	io.SetMousePosition(button)
	frame()
	io.SetMouseButtonDown(0, true)
	frame()
	io.SetMouseButtonDown(0, false)
	frame()
	if clicks != 1 {
		t.Fatalf("stale workspace focus cancelled the first toolbar click: clicks=%d", clicks)
	}
	preview := &focusTestContent{name: "Preview: Map"}
	previewWS := workspace.New(preview)
	area.addWorkspace(previewWS)
	previewWS.SetTriggerFocus(true)
	for i := 0; i < 4; i++ {
		frame()
	}
	if area.ActiveWorkspace() != previewWS || preview.frames == 0 {
		t.Fatalf("preview did not become active: active=%v, rendered frames=%d", area.ActiveWorkspace(), preview.frames)
	}
}
