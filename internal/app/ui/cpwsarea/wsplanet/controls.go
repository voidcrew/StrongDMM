package wsplanet

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/workshop"
	"sdmm/internal/app/window"
	"sdmm/internal/imguiext/style"
	"sdmm/internal/planet"
)

func textField(label, hint string, value *string) bool {
	imgui.Text(label)
	imgui.SetNextItemWidth(-1)
	return imgui.InputTextWithHint("##"+label, hint, value)
}
func combo(label, preview string) bool {
	imgui.Text(label)
	imgui.SetNextItemWidth(-1)
	return imgui.BeginCombo("##"+label, preview)
}
func slider(label string, value *float64, low, high float32, format string) bool {
	imgui.Text(label)
	imgui.SetNextItemWidth(-1)
	v := float32(*value)
	changed := imgui.SliderFloatV("##"+label, &v, low, high, format, 0)
	if changed {
		*value = float64(v)
	}
	return changed
}
func (w *Workspace) library() {
	workshop.Section("PLANET", style.Teal)
	name := "Choose a planet"
	if w.project != nil {
		name = w.project.State.Definition.Name
	}
	if combo("Open planet", name) {
		for _, d := range w.planetChoices() {
			label := d.Name
			if draft := w.drafts[d.Path]; draft != nil {
				dirty := draft.nameDirty || draft.project.Modified()
				if draft.project == w.project {
					dirty = w.currentModified()
				}
				if draft.project.UnsavedNew() {
					label += " (new, unsaved)"
				} else if dirty {
					label += " (unsaved changes)"
				}
			}
			if imgui.Selectable(label + "##" + d.Path) {
				w.switchPlanet(d.Path)
			}
		}
		imgui.EndCombo()
	}
	if workshop.Button("+ New planet", false) {
		w.startCreation()
	}
	if w.project == nil || w.creating {
		return
	}
	if w.project.UnsavedNew() {
		if workshop.Button("Cancel new planet...", false) {
			w.cancelling = true
		}
	}
	workshop.Gap()
	workshop.Section("BIOMES", style.Teal)
	if w.preview != nil {
		workshop.Muted(fmt.Sprintf("%d biomes in this planet", len(w.project.State.UsedBiomes())))
	}
	imgui.BeginChildV("biome-list", imgui.Vec2{Y: max(130*window.PointSize(), imgui.ContentRegionAvail().Y-280*window.PointSize())}, false, 0)
	for _, path := range w.project.State.VisibleBiomes() {
		b := w.project.State.Biomes[path]
		badge := ""
		if w.preview != nil && (w.mode == 0 || w.mode == 2) {
			badge = fmt.Sprintf("%.0f%%", float64(w.preview.Counts[path])*100/float64(len(w.preview.Cells)))
		}
		detail := "Surface"
		if b.Cave {
			detail = "Caves"
		}
		pos := imgui.CursorScreenPos()
		active := w.selected == path
		if w.mode == 2 && w.climateView.brush != "" {
			active = w.climateView.brush == path
		}
		if workshop.Row(path, b.Name, detail, badge, active, style.Teal, 38) {
			if w.selectBiome(path) && w.mode == 2 {
				w.climateView.brush = path
				w.climateView.painting = true
			}
		}
		w.sprite(w.biomeGround(b), imgui.Vec2{X: pos.X + 9*window.PointSize(), Y: pos.Y + 12*window.PointSize()}, 32*window.PointSize())
	}
	imgui.EndChild()
	if workshop.Button("+ Add biome", false) {
		w.addBiome()
	}
	if workshop.Button("Delete biome...", false) {
		w.beginDeleteBiome()
	}
	workshop.Gap()
	if workshop.Button("Planet settings", false) {
		w.deleting = false
		w.mode = 3
		w.narrowEditor = true
	}
	if workshop.Button("Review & save", w.currentModified()) {
		if w.commitName() {
			w.record()
			w.review = true
			w.cancelling = false
		}
	}
	if w.message != "" {
		workshop.Muted(w.message)
	}
}

func (w *Workspace) createForm() {
	workshop.Title("Build a planet")
	workshop.Muted("Give it a name, choose its environment, then build its biomes visually.")
	textField("Planet name", "e.g. Glasswood", &w.newName)
	if len(w.catalog.Planets) == 0 {
		return
	}
	if combo("Starting environment", w.catalog.Planets[w.base].Name) {
		for i, d := range w.catalog.Planets {
			if imgui.Selectable(d.Name) {
				w.base = i
			}
		}
		imgui.EndCombo()
	}
	workshop.Muted("The starting environment supplies the atmosphere, weather and overmap appearance.")
	imgui.Checkbox("Start with one empty ground biome", &w.blank)
	workshop.Gap()
	if workshop.Button("Create planet", true) {
		w.createPlanet()
	}
	if workshop.Button("Cancel", false) {
		w.creating = false
	}
	if w.message != "" {
		imgui.TextWrapped(w.message)
	}
}

func (w *Workspace) addBiome() {
	if !w.commitName() {
		return
	}
	s := &w.project.State
	original := s.Biomes[w.selected]
	b := original
	base := planet.BiomeType
	if b.Cave {
		base += "/cave"
	}
	for i := 1; ; i++ {
		path := fmt.Sprintf("%s/workshop_%s_biome_%d", base, planet.ID(s.Definition.Path), i)
		if _, ok := s.Biomes[path]; ok || w.catalog.Dme.Objects[path] != nil {
			continue
		}
		b.Path = path
		b.Name = fmt.Sprintf("New biome %d", i)
		break
	}
	b.Local, b.Created = true, true
	b.Parent = original.Parent
	// Clone the table slices: a new biome must not mutate its starting biome.
	temp := planet.Clone(planet.State{Biomes: map[string]planet.Biome{b.Path: b}})
	b = temp.Biomes[b.Path]
	s.Biomes[b.Path] = b
	w.project.RememberNewBiome(b.Path)
	w.selectBiome(b.Path)
	w.mode = 1
	w.message = "New biome created. Assign it to climate cells when it is ready."
}

func (w *Workspace) terrain() {
	g := &w.project.State.Definition.Settings
	workshop.Title("Terrain & seed")
	workshop.Muted("Keep a seed while you change the terrain. Reroll to explore another landscape.")
	slider("Biome scale", &g.Zoom, 10, 150, "%.0f tiles")
	workshop.Tooltip("Larger values make broader climate regions.")
	slider("Mountain threshold", &g.Mountain, 0, 1, "%.2f")
	workshop.Tooltip("Land above this height uses cave biomes. Lower values make more mountains.")
	slider("Initial rock coverage", &g.Closed, 0, 100, "%.0f%%")
	if imgui.CollapsingHeader("Cave shaping") {
		for _, field := range []struct {
			name  string
			value *int
			max   int32
		}{{"Smoothing passes", &g.Iterations, 50}, {"Birth limit", &g.Birth, 8}, {"Death limit", &g.Death, 8}} {
			imgui.Text(field.name)
			v := int32(*field.value)
			imgui.SetNextItemWidth(-1)
			if imgui.SliderInt("##"+field.name, &v, 0, field.max) {
				*field.value = int(v)
			}
		}
	}
	imgui.Text("Preview size")
	if w.project.State.Definition.Schema > 0 {
		workshop.Muted("In game: 123 x 123 tiles, including landing space.")
	}
	v := int32(w.project.State.Size)
	imgui.SetNextItemWidth(-1)
	if imgui.SliderInt("##size", &v, 24, 256) {
		w.project.State.Size = int(v)
		w.fit = true
	}
	if workshop.Button("Reroll landscape", false) {
		seed := uint32(time.Now().UnixNano())
		w.project.State.Seeds = planet.Seeds{Height: seed % 50001, Heat: (seed ^ 0x846ca68b) % 50001, Moisture: (seed ^ 0x9e3779b9) % 50001, Detail: seed}
	}
	if imgui.CollapsingHeader("Seed values") {
		for _, f := range []struct {
			name  string
			value *uint32
		}{{"Height", &w.project.State.Seeds.Height}, {"Heat", &w.project.State.Seeds.Heat}, {"Moisture", &w.project.State.Seeds.Moisture}, {"Detail", &w.project.State.Seeds.Detail}} {
			imgui.Text(f.name)
			v := fmt.Sprint(*f.value)
			imgui.SetNextItemWidth(-1)
			if imgui.InputText("##seed-"+f.name, &v) {
				var parsed uint32
				if _, err := fmt.Sscan(v, &parsed); err == nil {
					*f.value = parsed
				}
			}
		}
		workshop.Muted("Terrain uses rust-g Perlin noise. Cave fill and population use repeatable editor rolls; a game round also depends on world position and its random state.")
	}
	if workshop.Button("Back to biomes", false) {
		w.mode = 0
	}
}

func (w *Workspace) editor() {
	if w.deleting {
		w.deleteBiomeForm()
		return
	}
	if w.mode == 3 {
		if w.picking && (w.pickingRiver || w.pickingEnvironment) {
			w.picker()
			return
		}
		tabs := []string{"Terrain & seed", "Rivers", "Environment", "Ruins"}
		w.settingsTab = max(0, min(w.settingsTab, len(tabs)-1))
		if combo("Planet settings", tabs[w.settingsTab]) {
			for i, name := range tabs {
				if imgui.Selectable(name) {
					w.settingsTab = i
				}
			}
			imgui.EndCombo()
		}
		switch w.settingsTab {
		case 1:
			w.riverSettings()
		case 2:
			w.environmentSettings()
		case 3:
			w.ruinSettings()
		default:
			w.terrain()
		}
		return
	}
	b, ok := w.project.State.Biomes[w.selected]
	if !ok {
		return
	}
	if w.picking {
		w.picker()
		return
	}
	workshop.Section("EDIT BIOME", style.Teal)
	if textField("Biome name", "A short descriptive name", &w.biomeName) {
		w.nameDirty = true
	}
	if imgui.IsItemDeactivatedAfterEdit() {
		w.commitName()
		b = w.project.State.Biomes[w.selected]
	}
	workshop.Muted("Changes apply to this planet.")
	imgui.BeginDisabledV(!w.nameDirty && !w.project.CanDiscardBiome(w.selected))
	if workshop.Button("Discard biome changes", false) {
		w.discardBiomeChanges()
		b = w.project.State.Biomes[w.selected]
	}
	imgui.EndDisabled()
	workshop.Tooltip("Restore this biome's saved or starting name and contents. Undo can restore your edits.")
	if w.table >= len(b.Tables) {
		w.table = 0
	}
	if combo("Contents", b.Tables[w.table].Name) {
		for i, t := range b.Tables {
			if imgui.Selectable(fmt.Sprintf("%s (%d)", t.Name, len(t.Entries))) {
				w.table = i
				w.selectedEntry, w.revealEntry = "", false
			}
		}
		imgui.EndCombo()
	}
	t := b.Tables[w.table]
	if t.ChanceField != "" {
		chance := t.Chance
		if slider("Spawn chance", &chance, 0, 100, "%.1f%%") {
			w.local()
			b = w.project.State.Biomes[w.selected]
			b.Tables[w.table].Chance = chance
			w.project.State.Biomes[w.selected] = b
		}
		workshop.Tooltip("Chance per eligible tile. Features and creatures also obey spacing limits.")
	}
	if t.Field == "dangerous_mob_spawn_list" {
		workshop.Muted("Replacements used in dangerous overmap zones. The sample shows ordinary creatures.")
	}
	if t.Field == "megafauna_spawn_list" {
		workshop.Muted("Boss candidates used by the game's population budget. These are listed here but not scattered in the sample.")
	}
	workshop.Gap()
	total := 0.0
	for _, e := range t.Entries {
		total += e.Weight
	}
	for i, e := range t.Entries {
		imgui.PushIDInt(i)
		pos := imgui.CursorScreenPos()
		selected := w.selectedEntry == e.Path
		if workshop.Row("entry", w.itemName(e.Path), "Click to replace", fmt.Sprintf("%.0f%%", e.Weight/total*100), selected, style.Teal, 38) {
			w.pickerScope = 0
			w.pickingRiver = false
			w.pickingEnvironment = false
			w.picking = true
			w.replace = i
			w.filter = ""
		}
		if selected && w.revealEntry {
			imgui.SetScrollHereY(.5)
			w.revealEntry = false
		}
		w.sprite(e.Path, imgui.Vec2{X: pos.X + 9*window.PointSize(), Y: pos.Y + 12*window.PointSize()}, 32*window.PointSize())
		weight := float32(e.Weight)
		imgui.Text("Weight")
		imgui.SameLine()
		imgui.SetNextItemWidth(max(40, imgui.ContentRegionAvail().X-85*window.PointSize()))
		if imgui.DragFloatV("##weight", &weight, .1, .01, 100000, "%.2f", 0) {
			if weight > 0 && weight <= 100000 {
				w.local()
				b = w.project.State.Biomes[w.selected]
				b.Tables[w.table].Entries[i].Weight = float64(weight)
				w.project.State.Biomes[w.selected] = b
			}
		}
		workshop.Tooltip("Relative weight. A choice with weight 2 appears twice as often as one with weight 1.")
		imgui.SameLine()
		imgui.BeginDisabledV(len(t.Entries) == 1 && (t.Field == "open_turf_types" || t.Field == "closed_turf_types"))
		if imgui.Button("Remove") {
			if selected {
				w.selectedEntry = ""
			}
			w.local()
			b = w.project.State.Biomes[w.selected]
			entries := b.Tables[w.table].Entries
			b.Tables[w.table].Entries = append(entries[:i], entries[i+1:]...)
			w.project.State.Biomes[w.selected] = b
			imgui.EndDisabled()
			imgui.PopID()
			break
		}
		imgui.EndDisabled()
		imgui.PopID()
	}
	if workshop.Button("+ Add choice", true) {
		w.pickerScope = 0
		w.pickingRiver = false
		w.pickingEnvironment = false
		w.picking = true
		w.replace = -1
		w.filter = ""
	}
	if len(t.Entries) == 0 {
		workshop.Muted("No choices in this group yet.")
	}
	if imgui.CollapsingHeader("Source details") {
		imgui.TextWrapped(b.Parent)
	}
}
func (w *Workspace) local() {
	previous := w.selected
	w.selected = w.project.State.LocalBiome(w.selected)
	if w.climateView.brush == previous {
		w.climateView.brush = w.selected
	}
}
func (w *Workspace) biomeGround(b planet.Biome) string {
	for _, t := range b.Tables {
		if t.Field == "open_turf_types" && len(t.Entries) > 0 {
			return t.Entries[0].Path
		}
	}
	return ""
}
func (w *Workspace) itemName(path string) string {
	if name := w.terrainName(path); name != "" {
		return name
	}
	if path == planet.MegafaunaRoll {
		return "Megafauna roll"
	}
	if o := w.dme.Objects[path]; o != nil {
		name := o.Vars.TextV("name", "")
		if name != "" {
			return strings.ReplaceAll(strings.ReplaceAll(name, `\improper `, ""), `\proper `, "")
		}
	}
	return planet.Label(path)
}
func (w *Workspace) reviewPanel() {
	workshop.Title("Review your planet")
	changes, err := w.project.Changes()
	if err != nil {
		imgui.TextWrapped(err.Error())
	} else if len(changes) == 0 {
		workshop.Muted("All changes are saved.")
	} else {
		workshop.Muted("Save the biome definitions, climate layout and terrain settings to this project.")
		for _, change := range changes {
			rel, _ := filepath.Rel(w.catalog.Dme.RootDir, change.Path)
			label := "Update"
			if !change.Existed {
				label = "Create"
			}
			imgui.Text(label + "  " + filepath.ToSlash(rel))
		}
	}
	workshop.Gap()
	imgui.BeginDisabledV(err != nil || len(changes) == 0)
	if workshop.Button("Save planet", true) {
		w.saveCurrent()
	}
	imgui.EndDisabled()
	if workshop.Button("Back to building", false) {
		w.review = false
	}
	if w.message != "" {
		imgui.TextWrapped(w.message)
	}
}
