package wsship

import (
	"testing"

	"github.com/SpaiR/imgui-go"
)

// Runs in the native test's ImGui context. Opening a combo changes the current
// window to its popup, which a render-only check of closed controls cannot test.
func exerciseDropdowns(t *testing.T) {
	t.Helper()
	io := imgui.CurrentIO()
	defer io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
	for _, label := range []string{"Part to edit", "Ship variant", "Room: cargo", "Docking direction"} {
		selected := 0
		var button, option imgui.Vec2
		frame := func() bool {
			imgui.NewFrame()
			imgui.SetNextWindowPos(imgui.Vec2{X: 20, Y: 20})
			imgui.SetNextWindowSize(imgui.Vec2{X: 360, Y: 280})
			imgui.BeginV("Workshop dropdown regression", nil, imgui.WindowFlagsNoSavedSettings|imgui.WindowFlagsNoResize|imgui.WindowFlagsNoMove)
			imgui.BeginChild("dropdown-host")
			choices := []string{"First option", "Second option"}
			open := combo(label, choices[selected])
			if open {
				for i, choice := range choices {
					if imgui.SelectableV(choice, selected == i, 0, imgui.Vec2{}) {
						selected = i
					}
					if i == 1 {
						lo, hi := imgui.ItemRectMin(), imgui.ItemRectMax()
						option = imgui.Vec2{X: (lo.X + hi.X) / 2, Y: (lo.Y + hi.Y) / 2}
					}
				}
				imgui.EndCombo()
			} else {
				lo, hi := imgui.ItemRectMin(), imgui.ItemRectMax()
				button = imgui.Vec2{X: (lo.X + hi.X) / 2, Y: (lo.Y + hi.Y) / 2}
			}
			imgui.EndChild()
			imgui.End()
			imgui.Render()
			return open
		}
		for repeat := 0; repeat < 2; repeat++ {
			io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
			for i := 0; i < 3; i++ {
				frame()
			}
			io.SetMousePosition(button)
			frame()
			io.SetMouseButtonDown(0, true)
			frame()
			io.SetMouseButtonDown(0, false)
			if !frame() {
				t.Fatalf("%s did not open", label)
			}
			// ImGui measures a newly opened popup before positioning it.
			frame()
			io.SetMousePosition(option)
			frame()
			io.SetMouseButtonDown(0, true)
			frame()
			io.SetMouseButtonDown(0, false)
			frame()
			if selected != 1 {
				t.Fatalf("%s did not select its second option", label)
			}
			if frame() {
				t.Fatalf("%s did not close after selection", label)
			}
			selected = 0
		}
	}
}
