package tools

import (
	"testing"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap/canvas"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

type fillControl struct{ dragging bool }

func (c *fillControl) Dragging() bool { return c.dragging }

type fillState struct{ coord util.Point }

func (s *fillState) HoverOutOfBounds() bool      { return false }
func (s *fillState) HoveredTile() util.Point     { return s.coord }
func (s *fillState) LastHoveredTile() util.Point { return s.coord }

type fillEditor struct {
	editor
	dmm     *dmmap.Dmm
	prefab  *dmmprefab.Prefab
	commits int
}

func TestFillFirstClickAfterPanelSelection(t *testing.T) {
	context := imgui.CreateContext(nil)
	defer context.Destroy()
	io := imgui.CurrentIO()
	io.SetIniFilename("")
	io.SetDisplaySize(imgui.Vec2{X: 640, Y: 320})
	io.SetDeltaTime(1.0 / 60)
	io.Fonts().TextureDataRGBA32()
	previousCC, previousCS, previousED := cc, cs, ed
	previousActive, previousEnabled, previousCoord := active, enabled, oldCoord
	previousHandled := pressHandled
	previousTools, previousSelected, previousStarted := tools, selectedToolName, startedTool
	defer func() {
		cc, cs, ed = previousCC, previousCS, previousED
		active, enabled, oldCoord = previousActive, previousEnabled, previousCoord
		pressHandled = previousHandled
		tools, selectedToolName, startedTool = previousTools, previousSelected, previousStarted
	}()
	mutableVars := dmvars.MutableVariables{}
	vars := mutableVars.ToImmutable()
	coord := util.Point{X: 1, Y: 1, Z: 1}
	tile := &dmmap.Tile{Coord: coord, DefaultArea: dmmprefab.New(1, "/area", vars), DefaultTurf: dmmprefab.New(2, "/turf", vars)}
	editor := &fillEditor{dmm: &dmmap.Dmm{MaxX: 1, MaxY: 1, MaxZ: 1, Tiles: []*dmmap.Tile{tile}}}
	control := canvas.NewControl()
	control.AtCursor = true
	state := &fillState{coord: coord}
	cc, cs, ed = control, state, editor
	active, enabled, oldCoord = false, true, coord
	tools = map[string]Tool{TNFill: newFill()}
	selectedToolName, startedTool = TNFill, nil
	var panelItem imgui.Vec2
	wasFocused := false
	frame := func() {
		imgui.NewFrame()
		// Tools run at the beginning of the frame, before panels update focus.
		process(false)
		imgui.SetNextWindowPos(imgui.Vec2{})
		imgui.SetNextWindowSize(imgui.Vec2{X: 200, Y: 300})
		flags := imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoResize
		imgui.BeginV("Items", nil, flags)
		if imgui.Selectable("Chair") {
			editor.prefab = dmmprefab.New(3, "/obj/chair", vars)
		}
		lo, hi := imgui.ItemRectMin(), imgui.ItemRectMax()
		panelItem = imgui.Vec2{X: (lo.X + hi.X) / 2, Y: (lo.Y + hi.Y) / 2}
		imgui.End()
		imgui.SetNextWindowPos(imgui.Vec2{X: 220})
		imgui.SetNextWindowSize(imgui.Vec2{X: 400, Y: 300})
		imgui.BeginV("Map", nil, flags|imgui.WindowFlagsNoBringToFrontOnFocus)
		if control.Touched() && !imgui.IsWindowFocusedV(imgui.FocusedFlagsRootAndChildWindows) {
			imgui.SetWindowFocus()
		}
		focused := imgui.IsWindowFocusedV(imgui.FocusedFlagsRootAndChildWindows)
		control.Process(imgui.Vec2{X: 360, Y: 240})
		imgui.End()
		if focused && !wasFocused {
			// PaneMap.OnActivate rebinds the editor when returning from the item panel.
			SetEnabled(true)
			SetEditor(editor)
			SetCanvasState(state)
			SetCanvasControl(control)
		}
		wasFocused = focused
		imgui.Render()
	}
	for i := 0; i < 3; i++ {
		frame()
	}
	io.SetMousePosition(panelItem)
	frame()
	io.SetMouseButtonDown(0, true)
	frame()
	io.SetMouseButtonDown(0, false)
	frame()
	frame()
	if editor.prefab == nil || wasFocused {
		t.Fatal("item panel did not select the prefab and take focus")
	}
	io.SetMousePosition(imgui.Vec2{X: 300, Y: 100})
	frame()
	io.SetMouseButtonDown(0, true)
	for i := 0; i < 4; i++ {
		frame()
	}
	if editor.commits != 0 {
		t.Errorf("fill committed %d times before mouse release", editor.commits)
	}
	io.SetMouseButtonDown(0, false)
	for i := 0; i < 3; i++ {
		frame()
	}
	count := 0
	for _, instance := range tile.Instances() {
		if instance.Prefab() == editor.prefab {
			count++
		}
	}
	if count != 1 || editor.commits != 1 {
		t.Fatalf("first click after panel selection placed %d objects in %d fills, want 1 each", count, editor.commits)
	}
}

func (e *fillEditor) Dmm() *dmmap.Dmm                                   { return e.dmm }
func (e *fillEditor) SelectedPrefab() (*dmmprefab.Prefab, bool)         { return e.prefab, e.prefab != nil }
func (e *fillEditor) CommitChanges(string)                              { e.commits++ }
func (*fillEditor) OverlayPushArea(util.Bounds, util.Color, util.Color) {}

func TestFillOneStrokePerMousePress(t *testing.T) {
	context := imgui.CreateContext(nil)
	defer context.Destroy()
	for _, interruption := range []string{"none", "refocus", "save", "disable", "switch tool"} {
		t.Run(interruption, func(t *testing.T) {
			previousCC, previousCS, previousED := cc, cs, ed
			previousActive, previousEnabled, previousCoord := active, enabled, oldCoord
			previousHandled := pressHandled
			previousTools, previousSelected, previousStarted := tools, selectedToolName, startedTool
			defer func() {
				cc, cs, ed = previousCC, previousCS, previousED
				active, enabled, oldCoord = previousActive, previousEnabled, previousCoord
				pressHandled = previousHandled
				tools, selectedToolName, startedTool = previousTools, previousSelected, previousStarted
			}()
			mutableVars := dmvars.MutableVariables{}
			vars := mutableVars.ToImmutable()
			m := &dmmap.Dmm{MaxX: 2, MaxY: 2, MaxZ: 1}
			for y := 1; y <= 2; y++ {
				for x := 1; x <= 2; x++ {
					m.Tiles = append(m.Tiles, &dmmap.Tile{
						Coord:       util.Point{X: x, Y: y, Z: 1},
						DefaultArea: dmmprefab.New(1, "/area", vars),
						DefaultTurf: dmmprefab.New(2, "/turf", vars),
					})
				}
			}
			control := &fillControl{}
			state := &fillState{coord: util.Point{X: 1, Y: 1, Z: 1}}
			editor := &fillEditor{dmm: m, prefab: dmmprefab.New(3, "/obj/chair", vars)}
			cc, cs, ed = control, state, editor
			active, enabled, oldCoord = false, true, state.coord
			tools = map[string]Tool{TNFill: newFill(), TNRegion: &ToolRegion{}}
			selectedToolName, startedTool = TNFill, nil
			process(false)
			control.dragging = true
			process(false)
			state.coord = util.Point{X: 2, Y: 2, Z: 1}
			OnMouseMove()
			switch interruption {
			case "refocus":
				SetEditor(editor)
			case "save":
				FinishStroke()
			case "disable":
				SetEnabled(false)
				process(false)
				SetEnabled(true)
			case "switch tool":
				SetSelected(TNRegion)
				process(false)
				process(false)
				SetSelected(TNFill)
			}
			for i := 0; i < 3; i++ {
				process(false)
			}
			control.dragging = false
			process(false)
			assertObjects := func(tile *dmmap.Tile, want int) {
				t.Helper()
				count := 0
				for _, instance := range tile.Instances() {
					if instance.Prefab() == editor.prefab {
						count++
					}
				}
				if count != want {
					t.Errorf("tile %v has %d objects, want %d", tile.Coord, count, want)
				}
			}
			for _, tile := range m.Tiles {
				assertObjects(tile, 1)
			}
			if editor.commits != 1 {
				t.Errorf("one press committed %d fills, want 1", editor.commits)
			}
			// A deliberate second click must still place another object.
			control.dragging = true
			process(false)
			control.dragging = false
			process(false)
			assertObjects(m.GetTile(state.coord), 2)
			if editor.commits != 2 {
				t.Errorf("two presses committed %d fills, want 2", editor.commits)
			}
		})
	}
}
