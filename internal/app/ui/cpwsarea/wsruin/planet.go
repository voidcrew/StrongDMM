package wsruin

import (
	"fmt"
	"sort"
	"time"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/cpwsarea/wspreview"
	"sdmm/internal/app/ui/workshop"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/mappreview"
	"sdmm/internal/planet"
	"sdmm/internal/ruin"
)

func (ws *WsRuin) ruinTraits() map[string]bool {
	traits := map[string]bool{}
	if ws.selected == nil {
		return traits
	}
	for _, t := range append([]ruin.Template{*ws.selected}, ws.selected.Variants...) {
		if obj := ws.catalog.Dme.Objects[t.Type]; obj != nil {
			trait := obj.Vars.ValueV("ruin_type", "null")
			if trait != "null" {
				traits[trait] = true
			}
		}
	}
	return traits
}

func (ws *WsRuin) hasPlanetDestination() bool {
	if ws.planetCatalog != nil && ws.selected != nil {
		states := map[string]planet.State{}
		for _, d := range ws.planetCatalog.Planets {
			states[d.Path] = planet.State{Definition: d}
		}
		if ws.PlanetDrafts != nil {
			for _, s := range ws.PlanetDrafts() {
				states[s.Definition.Path] = s
			}
		}
		for _, s := range states {
			for _, t := range append([]ruin.Template{*ws.selected}, ws.selected.Variants...) {
				if ws.planetCatalog.AcceptsRuin(s, t.Type) {
					return true
				}
			}
		}
		return false
	}
	traits := ws.ruinTraits()
	for trait := range traits {
		if ws.planetTraits[trait] {
			return true
		}
	}
	return false
}

func (ws *WsRuin) hostPlanets() []planet.State {
	if ws.planetCatalog == nil {
		return nil
	}
	states := map[string]planet.State{}
	for _, d := range ws.planetCatalog.Planets {
		if p, err := planet.Open(ws.planetCatalog, d); err == nil {
			states[d.Path] = p.State
		}
	}
	if ws.PlanetDrafts != nil {
		for _, s := range ws.PlanetDrafts() {
			states[s.Definition.Path] = s
		}
	}
	var out []planet.State
	for _, s := range states {
		for _, t := range append([]ruin.Template{*ws.selected}, ws.selected.Variants...) {
			if ws.planetCatalog.AcceptsRuin(s, t.Type) {
				out = append(out, s)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Definition.Name < out[j].Definition.Name })
	return out
}

func (ws *WsRuin) beginPlanetView() {
	var err error
	ws.planetCatalog, err = planet.Discover(ws.app.LoadedEnvironment())
	if err != nil {
		ws.message = err.Error()
		return
	}
	choices := ws.hostPlanets()
	ws.planetHosts = choices
	if len(choices) == 0 {
		ws.message = "No readable planet matches this ruin's destination."
		return
	}
	ws.planetPath, ws.planetLevel = choices[0].Definition.Path, 1
	ws.planetSeeds = nil
	ws.planetCaves = false
	ws.viewOnPlanet, ws.planetRefresh = true, true
	ws.message = ""
}

func (ws *WsRuin) loadPreviewRuin() (*dmmap.Dmm, error) {
	if ws.LiveMap != nil {
		if live := ws.LiveMap(ws.selected.File); live != nil {
			copy := live.Copy()
			return &copy, nil
		}
	}
	data, err := dmmdata.New(ws.selected.File)
	if err != nil {
		return nil, err
	}
	source, _ := dmmap.New(ws.catalog.Dme, data, "")
	return source, nil
}

func (ws *WsRuin) refreshPlanetView(state planet.State) {
	ws.planetRefresh = false
	source, err := ws.loadPreviewRuin()
	if err == nil {
		ws.planetLevels = source.MaxZ
		ws.planetLevel = max(1, min(ws.planetLevel, source.MaxZ))
		state.Size = min(256, max(64, source.MaxX+24, source.MaxY+24))
		if ws.planetSeeds != nil {
			state.Seeds = *ws.planetSeeds
		}
		var terrain *planet.Preview
		terrain, err = planet.Generate(ws.planetCatalog, state, ws.catalog.Dme, planet.PreviewOptions{Populate: true, Caves: ws.planetCaves})
		if err == nil {
			var composite *dmmap.Dmm
			composite, err = planet.PlaceRuin(terrain.Map, source, ws.planetLevel)
			if err == nil {
				if ws.planetPreview == nil {
					ws.planetPreview = wspreview.NewWithOptions(composite, ws.catalog.Dme, mappreview.Options{Smoothing: true, PoweredFixtures: true, ExteriorLight: true})
				} else {
					ws.planetPreview.ReplaceSource(composite, ws.catalog.Dme)
				}
			}
		}
	}
	if err != nil {
		ws.message = err.Error()
	} else {
		ws.message = ""
	}
}

func (ws *WsRuin) planetView() {
	workshop.Title(ws.selected.Name + " on a planet")
	if imgui.Button("Back to ruin") {
		ws.viewOnPlanet = false
	}
	imgui.SameLine()
	if imgui.Button("Open map for editing") {
		ws.app.DoLoadResource(ws.selected.File)
	}
	imgui.SameLine()
	if imgui.Button("Refresh preview") {
		ws.planetRefresh = true
	}
	if ws.planetRefresh {
		ws.planetHosts = ws.hostPlanets()
	}
	choices := ws.planetHosts
	if len(choices) == 0 {
		hint("No readable planet matches this ruin's destination.")
		return
	}
	state := choices[0]
	for _, s := range choices {
		if s.Definition.Path == ws.planetPath {
			state = s
		}
	}
	if combo("Planet", state.Definition.Name) {
		for _, s := range choices {
			if imgui.Selectable(s.Definition.Name) {
				ws.planetPath, ws.planetSeeds = s.Definition.Path, nil
				ws.planetRefresh = true
				state = s
			}
		}
		imgui.EndCombo()
	}
	if len(state.Definition.Caves) > 0 {
		if imgui.Checkbox("Underground", &ws.planetCaves) {
			ws.planetRefresh = true
		}
		imgui.SameLine()
	}
	if ws.planetLevels > 1 {
		if combo("Ruin level", fmt.Sprint(ws.planetLevel)) {
			for level := 1; level <= ws.planetLevels; level++ {
				if imgui.Selectable(fmt.Sprint(level)) {
					ws.planetLevel, ws.planetRefresh = level, true
				}
			}
			imgui.EndCombo()
		}
	}
	if imgui.Button("Reroll surroundings") {
		seed := uint32(time.Now().UnixNano())
		ws.planetSeeds = &planet.Seeds{Height: seed % 50001, Heat: (seed ^ 0x846ca68b) % 50001, Moisture: (seed ^ 0x9e3779b9) % 50001, Detail: seed}
		ws.planetRefresh = true
	}
	if ws.planetRefresh {
		ws.refreshPlanetView(state)
	}
	hint("Ruin centered in seeded terrain. Refresh includes unsaved map and planet edits. Population is cleared inside its footprint; placement rules and runtime behavior are not simulated.")
	if ws.planetPreview != nil && ws.message == "" {
		imgui.PushID("ruin-planet-" + ws.selected.Type)
		ws.planetPreview.Process()
		imgui.PopID()
	}
}
