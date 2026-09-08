package style

import "github.com/SpaiR/imgui-go"

func RGB(hex uint32) imgui.Vec4 {
	return imgui.Vec4{X: float32(hex>>16&255) / 255, Y: float32(hex>>8&255) / 255, Z: float32(hex&255) / 255, W: 1}
}

var (
	Background = RGB(0x10151e)
	Surface    = RGB(0x19212d)
	Raised     = RGB(0x222e3d)
	Line       = RGB(0x344357)
	Text       = RGB(0xedf2f8)
	Muted      = RGB(0xa2b0c3)
	Teal       = RGB(0x59d9c5)
	Violet     = RGB(0xbba5ff)
	Amber      = RGB(0xf0c477)
	Danger     = RGB(0xff939b)
)
