package canvas

import (
	"testing"

	"github.com/SpaiR/imgui-go"
)

func TestCanvasClicksOnlyTouchHoveredCanvas(t *testing.T) {
	context := imgui.CreateContext(nil)
	defer context.Destroy()
	io := imgui.CurrentIO()
	io.SetIniFilename("")
	io.SetDisplaySize(imgui.Vec2{X: 320, Y: 240})
	io.SetDeltaTime(1.0 / 60)
	io.Fonts().TextureDataRGBA32()
	for button := 0; button < 3; button++ {
		io.SetMouseButtonDown(button, true)
		imgui.NewFrame()
		control := &Control{active: true}
		control.processMouseClick()
		if !control.Clicked() || !control.Touched() {
			t.Errorf("button %d did not touch the hovered canvas", button)
		}
		control.active = false
		control.processMouseClick()
		if control.Clicked() || control.Touched() {
			t.Errorf("button %d touched a different canvas", button)
		}
		imgui.Render()
		io.SetMouseButtonDown(button, false)
		imgui.NewFrame()
		control.processMouseClick()
		if control.Clicked() {
			t.Errorf("button %d remained clicked after release", button)
		}
		imgui.Render()
	}
}
