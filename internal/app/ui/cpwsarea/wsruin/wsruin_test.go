package wsruin

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SpaiR/imgui-go"
	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/platform"
	"sdmm/internal/ruin"
)

type testApp struct {
	dme    *dmenv.Dme
	opened string
}

func (a *testApp) LoadedEnvironment() *dmenv.Dme { return a.dme }
func (a *testApp) DoLoadResource(file string)    { a.opened = file }
func (a *testApp) SyncPrefabs()                  {}

func TestPropertyForm(t *testing.T) {
	p := ruin.Properties{Name: "A ruin", Description: "A description", Cost: 2.5, MineralCost: 1, Weight: .125, AllowDuplicates: true}
	f := makeForm(p)
	got, err := f.properties()
	if err != nil || got != p {
		t.Fatalf("lost properties: %+v %v", got, err)
	}
	for _, cost := range []string{"-1", "NaN", "Inf", "not a number"} {
		f.Cost = cost
		if _, err = f.properties(); err == nil {
			t.Fatal("accepted", cost)
		}
	}
	ws := &WsRuin{form: makeForm(p), initial: makeForm(p)}
	if ws.IsModified() {
		t.Fatal("clean form marked modified")
	}
	ws.form.Name = "Changed"
	if !ws.IsModified() {
		t.Fatal("property edits could be discarded on close")
	}
}

// Opt-in native rendering and authoring test. The game project is read only;
// generated files live in a separate fixture directory.
func TestNativeRuinWorkshop(t *testing.T) {
	path := os.Getenv("RUIN_TEST_DME")
	if path == "" {
		t.Skip("set RUIN_TEST_DME for native UI and project integration")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := glfw.Init(); err != nil {
		t.Fatal(err)
	}
	defer glfw.Terminate()
	glfw.WindowHint(glfw.Visible, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	const width, height = 1280, 900
	w, err := glfw.CreateWindow(width, height, "Ruin Workshop test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Destroy()
	w.MakeContextCurrent()
	if err = gl.Init(); err != nil {
		t.Fatal(err)
	}
	context := imgui.CreateContext(nil)
	defer context.Destroy()
	window.ApplyDefaultTheme()
	window.SetPointSize(1)
	io := imgui.CurrentIO()
	io.SetIniFilename("")
	io.SetDisplaySize(imgui.Vec2{X: width, Y: height})
	io.SetDisplayFrameBufferScale(imgui.Vec2{X: 1, Y: 1})
	io.SetDeltaTime(1.0 / 60)
	platform.InitImGuiGL()
	defer platform.DisposeImGuiGL()
	exerciseCombo(t)
	dme, err := dmenv.New(path)
	if err != nil {
		t.Fatal(err)
	}
	app := &testApp{dme: dme}
	ws := New(app)
	if ws.catalog == nil || len(ws.catalog.Templates) < 10 {
		t.Fatal("no live ruin catalog", ws.message)
	}
	t.Logf("Discovered %d ruins in %d locations", len(ws.catalog.Templates), len(ws.catalog.Locations))
	for _, item := range ws.catalog.Templates {
		if _, err := ruin.ReadProperties(dme.Objects[item.Type]); err != nil {
			t.Errorf("uneditable properties: %v", err)
		}
	}
	render := func() {
		gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
		gl.Viewport(0, 0, width, height)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		imgui.NewFrame()
		imgui.SetNextWindowPos(imgui.Vec2{})
		imgui.SetNextWindowSize(imgui.Vec2{X: width, Y: height})
		imgui.BeginV("Ruin Workshop", nil, imgui.WindowFlagsNoResize|imgui.WindowFlagsNoMove|imgui.WindowFlagsNoCollapse)
		ws.Process()
		imgui.End()
		imgui.Render()
		platform.Render(imgui.RenderedDrawData())
		gl.Finish()
	}
	output := os.Getenv("RUIN_TEST_OUTPUT")
	capture := func(name string) {
		for i := 0; i < 3; i++ {
			render()
		}
		if output == "" {
			return
		}
		if err := os.MkdirAll(output, 0755); err != nil {
			t.Fatal(err)
		}
		pixels := make([]byte, width*height*4)
		gl.ReadPixels(0, 0, width, height, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
		frame := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			copy(frame.Pix[y*frame.Stride:(y+1)*frame.Stride], pixels[(height-1-y)*width*4:(height-y)*width*4])
		}
		file, err := os.Create(filepath.Join(output, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err = png.Encode(file, frame); err != nil {
			t.Fatal(err)
		}
	}
	capture("ruin-library")
	ws.selectRuin(ws.catalog.Templates[0])
	capture("ruin-properties")
	if ws.project == nil {
		t.Fatal(ws.message)
	}
	ws.BeginNewRuin()
	ws.form.Name = "Ruin Workshop Test"
	ws.form.Description = "An abandoned survey camp."
	capture("ruin-name")
	if ws.id != "ruin_workshop_test" {
		t.Fatal("name did not produce an identifier", ws.id)
	}
	ws.step = 1
	capture("ruin-canvas")
	ws.step = 2
	capture("ruin-spawning")
	ws.step = 3
	capture("ruin-review")
	// Use a fresh project root so no game file can be changed by the creation.
	fixture := os.Getenv("RUIN_TEST_FIXTURE")
	if fixture == "" {
		fixture = t.TempDir()
	}
	if err = os.MkdirAll(fixture, 0755); err != nil {
		t.Fatal(err)
	}
	copyEnv := *dme
	copyEnv.RootDir = fixture
	copyEnv.RootFile = filepath.Join(fixture, "workshop.dme")
	if err = os.WriteFile(copyEnv.RootFile, []byte("#include \""+filepath.ToSlash(dme.RootFile)+"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	app.dme = &copyEnv
	ws.refresh()
	// The tests create both a planet ruin and a space ruin with opposite area settings.
	if !ws.Save() {
		t.Fatal(ws.message)
	}
	first := ws.selected.File
	if _, err = dmmdata.New(first); err != nil {
		t.Fatal(err)
	}
	ws.form.Cost = "2.5"
	if !ws.Save() {
		t.Fatal(ws.message)
	}
	ws.BeginNewRuin()
	for i, loc := range ws.catalog.Locations {
		if loc.Type == ruin.Type+"/space" {
			ws.chooseLocation(i)
		}
	}
	ws.form.Name = "Ruin Workshop Space Test"
	ws.form.Description = "A blank orbital ruin."
	ws.step = 0
	capture("ruin-space-name")
	ws.ground = 1
	ws.power = 1
	ws.gravity = false
	ws.step = 3
	if !ws.Save() {
		t.Fatal(ws.message)
	}
	if strings.EqualFold(first, ws.selected.File) {
		t.Fatal("new ruins shared a map")
	}
	rel, err := filepath.Rel(fixture, ws.selected.File)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatal("fixture escaped")
	}
	parsed, err := dmenv.New(copyEnv.RootFile)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Objects[ws.selected.Type] == nil {
		t.Fatal("created registration was not included")
	}
	dmmap.PrefabStorage.Free()
	dmmap.Init(parsed)
	data, err := dmmdata.New(ws.selected.File)
	if err != nil {
		t.Fatal(err)
	}
	m, unknown := dmmap.New(parsed, data, "")
	if len(unknown) != 0 || len(m.Tiles) != int(ws.width*ws.height) {
		t.Fatalf("generated map cannot open in the editor: %v", unknown)
	}
	t.Logf("Created and reloaded fixtures in %s", fixture)
}

func exerciseCombo(t *testing.T) {
	t.Helper()
	io := imgui.CurrentIO()
	defer io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
	var control, option imgui.Vec2
	selected := 0
	frame := func() bool {
		imgui.NewFrame()
		imgui.SetNextWindowPos(imgui.Vec2{X: 20, Y: 20})
		imgui.SetNextWindowSize(imgui.Vec2{X: 360, Y: 280})
		imgui.BeginV("Ruin choices", nil, imgui.WindowFlagsNoSavedSettings|imgui.WindowFlagsNoResize|imgui.WindowFlagsNoMove)
		imgui.BeginChild("host")
		open := combo("Ruin location", []string{"Beach", "Space"}[selected])
		if open {
			for i, name := range []string{"Beach", "Space"} {
				if imgui.Selectable(name) {
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
			control = imgui.Vec2{X: (lo.X + hi.X) / 2, Y: (lo.Y + hi.Y) / 2}
		}
		imgui.EndChild()
		imgui.End()
		imgui.Render()
		return open
	}
	for i := 0; i < 3; i++ {
		frame()
	}
	io.SetMousePosition(control)
	frame()
	io.SetMouseButtonDown(0, true)
	frame()
	io.SetMouseButtonDown(0, false)
	if !frame() {
		t.Fatal("location dropdown did not open")
	}
	frame()
	io.SetMousePosition(option)
	frame()
	io.SetMouseButtonDown(0, true)
	frame()
	io.SetMouseButtonDown(0, false)
	frame()
	if selected != 1 || frame() {
		t.Fatal("location selection did not close its dropdown")
	}
}
