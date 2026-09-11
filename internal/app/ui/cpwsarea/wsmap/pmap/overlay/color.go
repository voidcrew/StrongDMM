package overlay

import (
	"sdmm/internal/imguiext/style"
	"sdmm/internal/util"

	"github.com/SpaiR/imgui-go"
)

var (
	ColorEmpty = util.Color{}

	ColorToolAddTileFill      = util.MakeColor(1, 1, 1, 0.25)
	ColorToolAddAltTileFill   = util.MakeColor(1, 1, 1, 0.25)
	ColorToolAddTileBorder    = util.MakeColor(1, 1, 1, 1)
	ColorToolAddAltTileBorder = util.MakeColorFromVec4(style.ColorGold)

	ColorToolFillTileFill      = util.MakeColor(1, 1, 1, 0.25)
	ColorToolFillAltTileFill   = util.MakeColorFromVec4(style.ColorGold.Minus(imgui.Vec4{W: 0.75}))
	ColorToolFillTileBorder    = ColorEmpty
	ColorToolFillAltTileBorder = ColorEmpty

	ColorToolSelectTileFill   = util.MakeColor(1, 1, 1, 0.25)
	ColorToolSelectTileBorder = util.MakeColor(0, 1, 0, 1)

	ColorToolPickInstance = util.MakeColor(0, 1, 0, 1)

	ColorToolDeleteInstance      = util.MakeColor(1, 0, 0, 1)
	ColorToolDeleteAltTileFill   = util.MakeColor(1, 0, 0, 0.25)
	ColorToolDeleteAltTileBorder = util.MakeColorFromVec4(style.ColorGold)

	ColorToolReplaceInstance = util.MakeColor(0, 1, 0, 1)

	ColorFlickTileFill = util.MakeColor(1, 1, 1, 1)
	ColorFlickInstance = util.MakeColor(0, 1, 0, 1)

	ColorAreaBorder = util.MakeColor(1, 1, 1, 1)

	// Ship Workshop rooms and the room-shape tool.
	ColorRoomShapeFill    = util.MakeColor(0.25, 0.85, 0.95, 0.35)
	ColorRoomShapeBorder  = util.MakeColor(0.25, 0.85, 0.95, 1)
	ColorRoomShapeReject  = util.MakeColor(1, 0.2, 0.2, 0.7)
	ColorRoomActiveFill   = util.MakeColor(0.25, 0.85, 0.95, 0.10)
	ColorRoomActiveBorder = util.MakeColor(0.25, 0.85, 0.95, 0.9)
	ColorRoomOtherFill    = util.MakeColor(0.8, 0.6, 0.2, 0.08)
	ColorRoomOtherBorder  = util.MakeColor(0.9, 0.7, 0.25, 0.7)
	ColorRoomHoverFill    = util.MakeColor(0.8, 0.6, 0.2, 0.16)
	ColorRoomHoverBorder  = util.MakeColor(0.95, 0.75, 0.3, 0.9)
)
