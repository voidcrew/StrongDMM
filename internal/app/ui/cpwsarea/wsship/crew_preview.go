package wsship

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/ship"
)

type crewLayer struct {
	sprite *dmicon.Sprite
	color  uint32
	layer  int
}
type crewVisual struct {
	key      string
	layers   []crewLayer
	warnings []string
}
type wornSlot struct {
	id, file      string
	layer, hidden int
}

var wornSlots = []wornSlot{
	{"uniform", "under/default.dmi", 28, 4}, {"id", "id.dmi", 27, 0}, {"gloves", "hands.dmi", 24, 1}, {"shoes", "feet.dmi", 23, 8},
	{"accessory", "accessories.dmi", 28, 4},
	{"ears", "ears.dmi", 21, 32}, {"suit", "suits/default.dmi", 19, 0}, {"glasses", "eyes.dmi", 18, 64}, {"belt", "belt.dmi", 17, 16384},
	{"suit_store", "belt_mirror.dmi", 16, 2}, {"neck", "neck.dmi", 15, 1024}, {"back", "back.dmi", 14, 0},
	{"mask", "mask.dmi", 12, 16}, {"head", "head/default.dmi", 11, 2048}, {"l_hand", "", 7, 0}, {"r_hand", "", 7, 0},
}

func crewColor(s string) uint32 {
	if len(s) != 7 || s[0] != '#' {
		return 0xffffffff
	}
	v, e := strconv.ParseUint(s[1:], 16, 32)
	if e != nil {
		return 0xffffffff
	}
	return 0xff000000 | uint32(v&255)<<16 | uint32(v&0xff00) | uint32(v>>16)
}
func (v *crewVisual) add(file, state string, dir, layer int, color uint32) bool {
	dmi, e := dmicon.Cache.Get(file)
	if e != nil {
		return false
	}
	s := dmi.States[state]
	if s == nil || len(s.Sprites) == 0 || s.Frames == 0 {
		return false
	}
	v.layers = append(v.layers, crewLayer{s.SpriteV(dir), color, layer})
	return true
}

// GAGS configurations describe the real worn layers, separately from the map
// thumbnail. Support ordinary colorized icon-state layers without tinting trim.
func (v *crewVisual) greyscale(dme *dmenv.Dme, config, colors, state string, dir, layer int) bool {
	o := dme.Objects[config]
	if o == nil {
		return false
	}
	file := o.Vars.TextV("icon_file", "")
	source := o.Vars.TextV("json_config", "")
	path, e := ship.Inside(dme.RootDir, source)
	if e != nil {
		return false
	}
	data, e := os.ReadFile(path)
	if e != nil {
		return false
	}
	type part struct {
		Type, IconState, BlendMode string
		ColorIDs                   []int
	}
	var raw map[string][]struct {
		Type   string `json:"type"`
		State  string `json:"icon_state"`
		Blend  string `json:"blend_mode"`
		Colors []int  `json:"color_ids"`
	}
	if json.Unmarshal(data, &raw) != nil || len(raw[state]) == 0 {
		return false
	}
	old := len(v.layers)
	palette := strings.Split(strings.TrimPrefix(colors, "#"), "#")
	for _, p := range raw[state] {
		if p.Type != "icon_state" || (p.Blend != "" && p.Blend != "overlay") || len(p.Colors) > 1 {
			v.layers = v.layers[:old]
			return false
		}
		color := uint32(0xffffffff)
		if len(p.Colors) == 1 {
			n := p.Colors[0] - 1
			if n < 0 || n >= len(palette) {
				v.layers = v.layers[:old]
				return false
			}
			color = crewColor("#" + palette[n])
		}
		if !v.add(file, p.State, dir, layer, color) {
			v.layers = v.layers[:old]
			return false
		}
	}
	return true
}
func buildCrewVisual(p *ship.Project, j ship.CrewJob, dir int) crewVisual {
	v := crewVisual{}
	// A neutral human mannequin uses the same directional bodypart sprites as
	// the game. Hands sit above uniforms and below gloves.
	for _, s := range []string{"human_r_leg", "human_l_leg", "human_chest_m", "human_r_arm", "human_l_arm", "human_head_m"} {
		v.add("icons/mob/human/bodyparts_greyscale.dmi", s, dir, 34, 0xffb9b9b9)
	}
	for _, s := range []string{"human_r_hand", "human_l_hand"} {
		v.add("icons/mob/human/bodyparts_greyscale.dmi", s, dir, 25, 0xffb9b9b9)
	}
	equipment := p.CrewEquipment(j)
	hidden := 0
	for _, slot := range []string{"suit", "head", "mask"} {
		if o := p.Dme.Objects[equipment[slot]]; o != nil {
			hidden |= int(o.Vars.FloatV("flags_inv", 0))
		}
	}
	for _, slot := range wornSlots {
		path := equipment[slot.id]
		if slot.id == "accessory" && equipment["uniform"] == "" {
			continue
		}
		if path == "" || slot.hidden&hidden != 0 {
			continue
		}
		o := p.Dme.Objects[path]
		if o == nil {
			continue
		}
		file, state := o.Vars.TextV("worn_icon", ""), o.Vars.TextV("worn_icon_state", "")
		layer := int(o.Vars.FloatV("alternate_worn_layer", 0))
		if layer == 0 {
			layer = slot.layer
		}
		config := o.Vars.ValueV("greyscale_config_worn", "")
		if slot.id == "l_hand" || slot.id == "r_hand" {
			field := "lefthand_file"
			config = o.Vars.ValueV("greyscale_config_inhand_left", "")
			if slot.id == "r_hand" {
				field = "righthand_file"
				config = o.Vars.ValueV("greyscale_config_inhand_right", "")
			}
			file = o.Vars.TextV(field, "")
			state = o.Vars.TextV("inhand_icon_state", "")
		}
		if file == "" && slot.file != "" {
			file = "icons/mob/clothing/" + slot.file
		}
		if state == "" {
			state = o.Vars.TextV("icon_state", "")
		}
		if config != "" && config != "null" {
			if v.greyscale(p.Dme, config, o.Vars.TextV("greyscale_colors", ""), state, dir, layer) {
				continue
			}
			v.warnings = append(v.warnings, slot.id+": custom color layers unavailable")
		}
		if !v.add(file, state, dir, layer, crewColor(o.Vars.TextV("color", ""))) {
			v.warnings = append(v.warnings, slot.id+": no static worn sprite")
		}
	}
	sort.SliceStable(v.layers, func(i, j int) bool { return v.layers[i].layer > v.layers[j].layer })
	return v
}
func (ws *WsShip) crewMannequin() {
	c := &ws.crew
	for i, d := range []struct {
		name string
		dir  int
	}{{"Front", 2}, {"Back", 1}, {"Left", 8}, {"Right", 4}} {
		if i > 0 {
			imgui.SameLine()
		}
		if imgui.SmallButton(d.name) {
			c.direction = d.dir
		}
	}
	j := c.jobs[c.selected]
	b, _ := json.Marshal(j)
	key := fmt.Sprintf("%d:%s", c.direction, b)
	if ws.crewVisual.key != key {
		ws.crewVisual = buildCrewVisual(ws.project, j, c.direction)
		ws.crewVisual.key = key
	}
	size := min(224*window.PointSize(), imgui.ContentRegionAvail().X)
	pos := imgui.CursorScreenPos()
	imgui.Dummy(imgui.Vec2{X: size, Y: size})
	draw := imgui.WindowDrawList()
	draw.AddRectFilled(pos, imgui.Vec2{X: pos.X + size, Y: pos.Y + size}, 0xff252321)
	pixel := size / 40
	origin := imgui.Vec2{X: pos.X + 4*pixel, Y: pos.Y + 4*pixel}
	for _, l := range ws.crewVisual.layers {
		s := l.sprite
		draw.AddImageV(imgui.TextureID(s.Texture()), origin, imgui.Vec2{X: origin.X + float32(s.IconWidth())*pixel, Y: origin.Y + float32(s.IconHeight())*pixel}, imgui.Vec2{X: s.U1, Y: s.V1}, imgui.Vec2{X: s.U2, Y: s.V2}, imgui.PackedColor(l.color))
	}
	hint("Live human preview")
	tooltip("Uses the game's worn sprites. Species, animated equipment, runtime overlays and post-equip effects can change the in-game appearance. Bag contents and pockets are not visible.")
	if len(ws.crewVisual.warnings) > 0 {
		if imgui.CollapsingHeader(fmt.Sprintf("%d preview details", len(ws.crewVisual.warnings))) {
			for _, w := range ws.crewVisual.warnings {
				hint(w)
			}
		}
	}
}
