package menu

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/SpaiR/imgui-go"
	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"sdmm/internal/app/selfupdate"
	"sdmm/internal/app/window"
	"sdmm/internal/env"
	"sdmm/internal/platform"
)

type updatePopupApp struct{ app }

func (*updatePopupApp) DoSelfUpdate()                            {}
func (*updatePopupApp) DoRestart()                               {}
func (*updatePopupApp) DoIgnoreUpdate()                          {}
func (*updatePopupApp) DoCheckForUpdates()                       {}
func (*updatePopupApp) DoSelectUpdateChannel(selfupdate.Channel) {}
func (*updatePopupApp) DoOpenUpdateDownload()                    {}

func TestNativeUpdatePopup(t *testing.T) {
	output := os.Getenv("VOIDWORKS_TEST_UPDATE_UI")
	if output == "" {
		t.Skip("set VOIDWORKS_TEST_UPDATE_UI for native popup captures")
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
	const width, height = 860, 500
	handle, err := glfw.CreateWindow(width, height, "Update popup test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Destroy()
	handle.MakeContextCurrent()
	if err := gl.Init(); err != nil {
		t.Fatal(err)
	}
	context := imgui.CreateContext(nil)
	defer context.Destroy()
	window.ApplyDefaultTheme()
	io := imgui.CurrentIO()
	io.SetIniFilename("")
	io.SetDisplaySize(imgui.Vec2{X: width, Y: height})
	io.SetDisplayFrameBufferScale(imgui.Vec2{X: 1, Y: 1})
	io.SetDeltaTime(1.0 / 60)
	platform.InitImGuiGL()
	defer platform.DisposeImGuiGL()
	window.SetPointSize(1)
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	previousVersion := env.Version
	defer func() { env.Version = previousVersion }()
	for _, test := range []struct {
		name               string
		status             upStatus
		installed, offered string
		channel            selfupdate.Channel
	}{
		{"available", upStatusAvailable, "0.5.1", "0.5.2", selfupdate.Stable},
		{"downloading", upStatusUpdating, "0.5.1", "0.5.2", selfupdate.Stable},
		{"ready", upStatusUpdated, "0.5.1", "0.5.2", selfupdate.Stable},
		{"current", upStatusCurrent, "0.5.2", "0.5.2", selfupdate.Stable},
		{"error", upStatusError, "0.5.1", "", selfupdate.Stable},
		{"switch-to-beta", upStatusAvailable, "0.5.13", "0.5.14-beta.1", selfupdate.Beta},
		{"beta-ready", upStatusUpdated, "0.5.13", "0.5.14-beta.1", selfupdate.Beta},
		{"return-to-stable", upStatusAvailable, "0.5.14-beta.1", "0.5.13", selfupdate.Stable},
		{"stable-ready", upStatusUpdated, "0.5.14-beta.1", "0.5.13", selfupdate.Stable},
	} {
		env.Version = test.installed
		status := test.status
		m := &Menu{app: &updatePopupApp{}, updateStatus: status, updateVersion: test.offered, updateChannel: test.channel}
		if status != upStatusCurrent {
			m.updateDescription = "Voidworks now downloads and verifies updates in the background.\n\nChoose Update & restart when you are ready. Your project reopens after the update.\n\nWindows x64 package with the matching editor, checker, and source."
		}
		if status == upStatusError {
			m.updateError = "The installation folder must be writable. Extract Voidworks to a folder you own, then try again."
		}
		m.ShowUpdatePopup()
		for i := 0; i < 4; i++ {
			gl.Clear(gl.COLOR_BUFFER_BIT)
			imgui.NewFrame()
			imgui.SetNextWindowPos(imgui.Vec2{})
			imgui.SetNextWindowSize(imgui.Vec2{X: width, Y: height})
			imgui.BeginV("Voidworks", nil, imgui.WindowFlagsNoSavedSettings|imgui.WindowFlagsNoResize|imgui.WindowFlagsNoMove)
			m.showUpdateMenu()
			if !imgui.IsPopupOpen("update_menu") {
				t.Fatal("manual update status did not open")
			}
			imgui.End()
			imgui.Render()
			platform.Render(imgui.RenderedDrawData())
		}
		pixels := make([]byte, width*height*4)
		gl.ReadPixels(0, 0, width, height, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
		frame := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			copy(frame.Pix[y*frame.Stride:(y+1)*frame.Stride], pixels[(height-1-y)*width*4:(height-y)*width*4])
		}
		file, err := os.Create(filepath.Join(output, test.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, frame); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
