package pmap

import (
	"testing"

	"github.com/SpaiR/imgui-go"
	"github.com/go-gl/glfw/v3.3/glfw"
	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap/canvas"
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
)

func TestTypingDoesNotActivateMappingTools(t *testing.T) {
	context := imgui.CreateContext(nil)
	defer context.Destroy()
	io := imgui.CurrentIO()
	io.SetIniFilename("")
	io.SetDisplaySize(imgui.Vec2{X: 640, Y: 320})
	io.SetDeltaTime(1.0 / 60)
	io.Fonts().TextureDataRGBA32()
	previousActive, previousLast := activePane, lastActivePane
	previousMode, previousLastTool, previousPrevTool := tmpToolIsInTemporalMode, tmpToolLastSelectedName, tmpToolPrevSelectedName
	previousTool := tools.Selected().Name()
	defer func() {
		activePane, lastActivePane = previousActive, previousLast
		tmpToolIsInTemporalMode, tmpToolLastSelectedName, tmpToolPrevSelectedName = previousMode, previousLastTool, previousPrevTool
		tools.SetSelected(previousTool)
	}()
	pane := &PaneMap{canvasControl: canvas.NewControl()}
	pane.canvasControl.AtCursor = true
	activePane, lastActivePane = nil, pane
	tmpToolIsInTemporalMode, tmpToolLastSelectedName, tmpToolPrevSelectedName = false, "", ""
	tools.SetSelected(tools.TNFill)
	var inputPosition imgui.Vec2
	var value string
	var inputActive bool
	frame := func() {
		imgui.NewFrame()
		processTempToolsMode()
		flags := imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoResize
		imgui.SetNextWindowPos(imgui.Vec2{})
		imgui.SetNextWindowSize(imgui.Vec2{X: 200, Y: 300})
		imgui.BeginV("Properties", nil, flags)
		imgui.InputText("Name", &value)
		inputActive = imgui.IsItemActive()
		lo, hi := imgui.ItemRectMin(), imgui.ItemRectMax()
		inputPosition = imgui.Vec2{X: (lo.X + hi.X) / 2, Y: (lo.Y + hi.Y) / 2}
		imgui.End()
		imgui.SetNextWindowPos(imgui.Vec2{X: 220})
		imgui.SetNextWindowSize(imgui.Vec2{X: 400, Y: 300})
		imgui.BeginV("Map", nil, flags)
		pane.canvasControl.Process(imgui.Vec2{X: 360, Y: 240})
		pane.focusCanvas()
		imgui.End()
		imgui.Render()
	}
	for i := 0; i < 3; i++ {
		frame()
	}
	io.SetMousePosition(inputPosition)
	frame()
	io.SetMouseButtonDown(0, true)
	frame()
	io.SetMouseButtonDown(0, false)
	frame()
	if !inputActive {
		t.Fatal("click did not focus the text field")
	}
	// Leave the field focused while moving the pointer back over the map.
	io.SetMousePosition(imgui.Vec2{X: 300, Y: 100})
	frame()
	frame()
	for _, key := range []glfw.Key{glfw.KeyS, glfw.KeyD, glfw.KeyR, glfw.KeySpace} {
		io.KeyPress(int(key))
		io.AddInputCharacters(string(rune(key)))
		for i := 0; i < 3; i++ {
			frame()
			if tools.Selected().Name() != tools.TNFill {
				t.Errorf("typing key %v selected %s", key, tools.Selected().Name())
			}
			if pane.canvasControl.Moving() {
				t.Errorf("typing key %v started panning", key)
			}
		}
		io.KeyRelease(int(key))
		frame()
		if !inputActive {
			t.Errorf("typing key %v stole text-field focus", key)
		}
	}
	if value != "SDR " {
		t.Errorf("text field contains %q, want SDR and a space", value)
	}
	// Clicking the canvas ends text entry; the normal S picker must still work.
	io.SetMouseButtonDown(0, true)
	frame()
	io.SetMouseButtonDown(0, false)
	frame()
	frame()
	io.KeyPress(int(glfw.KeyS))
	frame()
	if tools.Selected().Name() != tools.TNPick {
		t.Error("S did not activate the picker outside text entry")
	}
	io.KeyRelease(int(glfw.KeyS))
	frame()
	if tools.Selected().Name() != tools.TNFill {
		t.Error("releasing S did not restore Fill")
	}
	io.KeyPress(int(glfw.KeySpace))
	frame()
	if !pane.canvasControl.Moving() {
		t.Error("Space did not pan outside text entry")
	}
	io.KeyRelease(int(glfw.KeySpace))
	frame()
	// Re-entering the field must retain focus after panning has ended.
	io.SetMousePosition(inputPosition)
	frame()
	io.SetMouseButtonDown(0, true)
	frame()
	io.SetMouseButtonDown(0, false)
	for i := 0; i < 3; i++ {
		frame()
	}
	if !inputActive {
		t.Error("text field did not retain focus after panning")
	}
}
