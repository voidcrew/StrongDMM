package wsship

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/ship"
)

type crewEditor struct {
	scope                                   string
	jobs                                    []ship.CrewJob
	scopes                                  []ship.CrewScope
	selected                                int
	slot                                    int
	filter, outfitFilter, lastPrefab, error string
	items, outfits                          []string
	direction                               int
	dirty                                   bool
	contents                                int // 0 equipment, 1 backpack, 2 belt
}

func (ws *WsShip) beginCrew() {
	ws.flush()
	ws.OnFocusChange(false)
	ws.task = taskCrew
	ws.crew = crewEditor{selected: -1, direction: 2}
	ws.crew.scopes = ws.project.CrewScopes()
	ws.crewVisual = crewVisual{}
	for path, o := range ws.project.Dme.Objects {
		if strings.HasPrefix(path, "/obj/item/") {
			ws.crew.items = append(ws.crew.items, path)
		}
		if strings.HasPrefix(path, "/datum/outfit/") && ws.project.Dme.Objects[o.Vars.ValueV("jobtype", "")] != nil && !strings.Contains(path, "/workshop_") {
			ws.crew.outfits = append(ws.crew.outfits, path)
		}
	}
	sort.Strings(ws.crew.items)
	sort.Strings(ws.crew.outfits)
	ws.loadCrewScope("ship")
	tools.SetEnabled(false)
}
func (ws *WsShip) loadCrewScope(scope string) {
	jobs, err := ws.project.CrewJobs(scope)
	if err != nil {
		ws.crew.error = err.Error()
		return
	}
	ws.crew.scope, ws.crew.jobs, ws.crew.selected, ws.crew.dirty = scope, jobs, -1, false
	if len(jobs) > 0 {
		ws.crew.selected = 0
	}
	ws.armCrewPicker()
	ws.crew.error = ""
}
func (ws *WsShip) armCrewPicker() {
	ws.crew.lastPrefab = ""
	if p, ok := ws.app.SelectedPrefab(); ok {
		ws.crew.lastPrefab = p.Path()
	}
}
func (ws *WsShip) commitCrew() bool {
	if ws.task != taskCrew || !ws.crew.dirty {
		return true
	}
	if e := ws.project.ValidateCrew(ws.crew.jobs); e != nil {
		ws.crew.error = e.Error()
		return false
	}
	ws.message = ""
	ws.change("Edit crew roster", func() error { return ws.project.SetCrewJobs(ws.crew.scope, ws.crew.jobs) })
	if ws.message != "" {
		ws.crew.error = ws.message
		return false
	}
	jobs, e := ws.project.CrewJobs(ws.crew.scope)
	if e != nil {
		ws.crew.error = e.Error()
		return false
	}
	ws.crew.jobs, ws.crew.dirty, ws.crew.error = jobs, false, ""
	tools.SetEnabled(false)
	return true
}
func (ws *WsShip) crewControls() {
	c := &ws.crew
	if actionButton("< Back to ship", false) && ws.commitCrew() {
		ws.finishTask()
		ws.OnFocusChange(true)
		return
	}
	heading("CREW & EQUIPMENT")
	preview := "Ship crew"
	for _, s := range c.scopes {
		if s.ID == c.scope {
			preview = s.Name
		}
	}
	if comboHelp("Jobs belong to", preview, "Ship crew is the base roster. A variant can replace it. Room options add jobs when installed.") {
		for _, s := range c.scopes {
			if imgui.SelectableV(s.Name, c.scope == s.ID, 0, imgui.Vec2{}) && ws.commitCrew() {
				ws.loadCrewScope(s.ID)
			}
		}
		imgui.EndCombo()
	}
	if strings.HasPrefix(c.scope, "theme/") {
		hint("An empty variant roster uses the ship's crew.")
	}
	if strings.HasPrefix(c.scope, "module/") {
		hint("These slots are added when this room option is installed.")
	}
	space()
	total := 0
	for _, j := range c.jobs {
		total += j.Slots
	}
	imgui.Text(fmt.Sprintf("%d jobs · %d slots", len(c.jobs), total))
	if actionButton("+ Create job", true) && ws.commitCrew() {
		name := "New job"
		for n := 2; ; n++ {
			used := false
			for _, j := range c.jobs {
				if strings.EqualFold(strings.TrimSpace(j.Name), name) {
					used = true
				}
			}
			if !used {
				break
			}
			name = fmt.Sprintf("New job %d", n)
		}
		c.jobs = append(c.jobs, ship.CrewJob{Name: name, Slots: 1, Category: "Assistant", Outfit: "/datum/outfit/job/assistant"})
		c.selected = len(c.jobs) - 1
		c.dirty = true
		ws.commitCrew()
		ws.armCrewPicker()
	}
	if strings.HasPrefix(c.scope, "theme/") && len(c.jobs) == 0 && actionButton("Copy ship crew into this variant", false) {
		jobs, e := ws.project.CrewJobs("ship")
		if e != nil {
			c.error = e.Error()
		} else {
			for i := range jobs {
				jobs[i].ID = ""
			}
			c.jobs = jobs
			c.selected = 0
			c.dirty = true
			ws.commitCrew()
		}
	}
	space()
	imgui.BeginChildV("crew-job-list", imgui.Vec2{Y: max(100, imgui.ContentRegionAvail().Y-90*window.PointSize())}, false, 0)
	for i, j := range c.jobs {
		imgui.PushIDInt(i)
		if imgui.SelectableV(fmt.Sprintf("%s  × %d", j.Name, j.Slots), c.selected == i, 0, imgui.Vec2{Y: 32 * window.PointSize()}) && ws.commitCrew() {
			c.selected = i
			ws.armCrewPicker()
		}
		imgui.PopID()
	}
	imgui.EndChild()
	if c.error != "" {
		imgui.TextWrapped(c.error)
	} else {
		hint("Changes join the ship's undo history. Review & save writes the files.")
	}
}
func (ws *WsShip) crewItemName(path string) string {
	if path == "" {
		return "Empty"
	}
	if o := ws.project.Dme.Objects[path]; o != nil {
		if name := o.Vars.TextV("name", ""); name != "" {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(name, `\improper `), `\proper `))
		}
	}
	return path
}
func (ws *WsShip) crewContent() {
	c := &ws.crew
	if c.selected < 0 || c.selected >= len(c.jobs) {
		title("Set up your crew")
		hint("Create a job on the left, then choose its number of slots and starting equipment.")
		return
	}
	if p, ok := ws.app.SelectedPrefab(); ok && p.Path() != c.lastPrefab {
		c.lastPrefab = p.Path()
		if strings.HasPrefix(p.Path(), "/obj/item/") {
			ws.chooseCrewItem(p.Path())
		}
	}
	scale := window.PointSize()
	available := imgui.ContentRegionAvail().X
	imgui.BeginChildV("crew-details", imgui.Vec2{X: min(255*scale, available*.29)}, false, 0)
	ws.crewDetails()
	imgui.EndChild()
	if c.selected < 0 || c.selected >= len(c.jobs) {
		return
	}
	imgui.SameLine()
	imgui.BeginChildV("crew-mannequin", imgui.Vec2{X: min(265*scale, available*.29)}, false, 0)
	title("Equipment")
	ws.crewMannequin()
	space()
	hint("Choose a slot to change its item.")
	equipment := ws.project.CrewEquipment(c.jobs[c.selected])
	imgui.BeginChild("crew-slots")
	for i, s := range ship.EquipmentSlots {
		imgui.PushID(s.ID)
		if imgui.SelectableV(s.Name+": "+ws.crewItemName(equipment[s.ID]), c.contents == 0 && c.slot == i, 0, imgui.Vec2{Y: 26 * scale}) {
			c.slot = i
			c.contents = 0
			ws.armCrewPicker()
		}
		tooltip(equipment[s.ID])
		imgui.PopID()
	}
	imgui.EndChild()
	imgui.EndChild()
	imgui.SameLine()
	imgui.BeginChild("crew-item-picker")
	ws.crewPicker()
	imgui.EndChild()
}
func (ws *WsShip) crewDetails() {
	c := &ws.crew
	j := &c.jobs[c.selected]
	title("Job slot")
	if textField("Job name", "e.g. Salvage Engineer", &j.Name) {
		c.dirty = true
	}
	if imgui.IsItemDeactivatedAfterEdit() {
		ws.commitCrew()
		j = &c.jobs[c.selected]
	}
	slots := int32(j.Slots)
	numberField("Number of slots", &slots)
	if int(slots) != j.Slots {
		j.Slots = int(slots)
		c.dirty = true
	}
	if imgui.IsItemDeactivatedAfterEdit() {
		ws.commitCrew()
		j = &c.jobs[c.selected]
	}
	if combo("Category", j.Category) {
		for _, category := range ship.CrewCategories {
			if imgui.Selectable(category) {
				j.Category = category
				c.dirty = true
			}
		}
		imgui.EndCombo()
		if c.dirty {
			ws.commitCrew()
			j = &c.jobs[c.selected]
		}
	}
	if imgui.Checkbox("Officer", &j.Officer) {
		c.dirty = true
		ws.commitCrew()
		j = &c.jobs[c.selected]
	}
	tooltip("Marks this job as an officer in the ship roster.")
	space()
	if comboHelp("Starting job outfit", ws.crewItemName(ws.project.CrewOutfit(*j)), "Supplies the underlying job, ID access and default gear. Your equipment selections override this preset.") {
		textField("Find an outfit", "Name or type path", &c.outfitFilter)
		for _, path := range c.outfits {
			if !strings.Contains(strings.ToLower(ws.crewItemName(path)+" "+path), strings.ToLower(c.outfitFilter)) {
				continue
			}
			if imgui.Selectable(ws.crewItemName(path) + "##" + path) {
				j.Outfit = path
				j.BaseOutfit = ""
				j.Equipment = nil
				j.Backpack = nil
				j.Belt = nil
				c.dirty = true
			}
			tooltip(path)
		}
		imgui.EndCombo()
		if c.dirty {
			ws.commitCrew()
			j = &c.jobs[c.selected]
		}
	}
	if o := ws.project.Dme.Objects[ws.project.CrewOutfit(*j)]; o != nil {
		hint("Job: " + ws.crewItemName(o.Vars.ValueV("jobtype", "")))
	}
	space()
	heading("CARRIED ITEMS")
	if actionButton("Backpack contents...", c.contents == 1) {
		c.contents = 1
		ws.armCrewPicker()
	}
	if actionButton("Belt contents...", c.contents == 2) {
		c.contents = 2
		ws.armCrewPicker()
	}
	hint("Pockets and hands are equipment slots. Bag contents are extra items placed inside the selected container.")
	space()
	if c.dirty {
		if actionButton("Apply changes", true) {
			ws.commitCrew()
		}
		if actionButton("Discard unfinished changes", false) {
			ws.loadCrewScope(c.scope)
		}
	}
	if actionButton("Delete this job...", false) {
		imgui.OpenPopup("Delete crew job")
	}
	if imgui.BeginPopupModal("Delete crew job") {
		imgui.TextWrapped("Remove " + j.Name + " from this roster? You can undo this change.")
		if imgui.Button("Remove job") {
			c.jobs = append(c.jobs[:c.selected], c.jobs[c.selected+1:]...)
			c.selected = min(c.selected, len(c.jobs)-1)
			c.dirty = true
			ws.commitCrew()
			imgui.CloseCurrentPopup()
		}
		imgui.SameLine()
		if imgui.Button("Cancel") {
			imgui.CloseCurrentPopup()
		}
		imgui.EndPopup()
	}
}
func (ws *WsShip) crewItemFits(path string) bool {
	c := &ws.crew
	o := ws.project.Dme.Objects[path]
	if o == nil || !strings.HasPrefix(path, "/obj/item/") {
		return false
	}
	if c.contents != 0 {
		return true
	}
	s := ship.EquipmentSlots[c.slot]
	if s.ID == "accessory" {
		return strings.HasPrefix(path, "/obj/item/clothing/accessory/")
	}
	return s.Flag == 0 || int(o.Vars.FloatV("slot_flags", 0))&s.Flag != 0
}
func (ws *WsShip) chooseCrewItem(path string) {
	c := &ws.crew
	if c.selected < 0 || c.selected >= len(c.jobs) {
		return
	}
	if !ws.crewItemFits(path) {
		c.error = "That item cannot go in " + ship.EquipmentSlots[c.slot].Name + ". Choose a matching item or another slot."
		return
	}
	j := &c.jobs[c.selected]
	if c.contents == 0 {
		if j.Equipment == nil {
			j.Equipment = map[string]string{}
		}
		j.Equipment[ship.EquipmentSlots[c.slot].ID] = path
	} else {
		bag, e := ws.project.CrewContents(*j, c.contents == 2)
		if e != nil {
			c.error = e.Error()
			return
		}
		bag[path]++
		if c.contents == 1 {
			j.Backpack = bag
		} else {
			j.Belt = bag
		}
	}
	c.dirty = true
	ws.commitCrew()
	ws.app.DoSelectPrefab(dmmap.PrefabStorage.Initial(path))
	c.lastPrefab = path
}
func (ws *WsShip) crewPicker() {
	c := &ws.crew
	if c.selected < 0 || c.selected >= len(c.jobs) {
		return
	}
	j := &c.jobs[c.selected]
	label := ship.EquipmentSlots[c.slot].Name
	if c.contents > 0 {
		label = []string{"", "Backpack contents", "Belt contents"}[c.contents]
	}
	title(label)
	if c.contents == 0 {
		equipment := ws.project.CrewEquipment(*j)
		hint(ws.crewItemName(equipment[ship.EquipmentSlots[c.slot].ID]))
		if imgui.Button("Empty slot") {
			if j.Equipment == nil {
				j.Equipment = map[string]string{}
			}
			j.Equipment[ship.EquipmentSlots[c.slot].ID] = ""
			c.dirty = true
			ws.commitCrew()
			j = &c.jobs[c.selected]
		}
		imgui.SameLine()
		if imgui.Button("Use preset") {
			delete(j.Equipment, ship.EquipmentSlots[c.slot].ID)
			c.dirty = true
			ws.commitCrew()
			j = &c.jobs[c.selected]
		}
	} else {
		bag, e := ws.project.CrewContents(*j, c.contents == 2)
		if e != nil {
			hint(e.Error())
		} else {
			keys := []string{}
			for k := range bag {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			imgui.BeginChildV("bag-contents", imgui.Vec2{Y: 150 * window.PointSize()}, true, 0)
			for _, path := range keys {
				imgui.PushID(path)
				n := int32(bag[path])
				imgui.SetNextItemWidth(90 * window.PointSize())
				if imgui.InputInt("##quantity", &n) {
					if n <= 0 {
						delete(bag, path)
					} else {
						bag[path] = int(n)
					}
					if c.contents == 1 {
						j.Backpack = bag
					} else {
						j.Belt = bag
					}
					c.dirty = true
				}
				imgui.SameLine()
				imgui.Text(ws.crewItemName(path))
				imgui.PopID()
			}
			imgui.EndChild()
			if c.dirty && !imgui.IsAnyItemActive() {
				ws.commitCrew()
				j = &c.jobs[c.selected]
			}
		}
		hint("Pick an item below to add one. Set its quantity to zero to remove it.")
	}
	space()
	textField("Find an item", "Search item names or type paths", &c.filter)
	hint("Or select an item in the Environment panel. It goes into the highlighted slot immediately.")
	if p, ok := ws.app.SelectedPrefab(); ok && ws.crewItemFits(p.Path()) {
		if actionButton("Use selected: "+ws.crewItemName(p.Path()), false) {
			ws.chooseCrewItem(p.Path())
		}
	}
	space()
	imgui.BeginChild("crew-search-results")
	count := 0
	for _, path := range c.items {
		if !ws.crewItemFits(path) || !strings.Contains(strings.ToLower(ws.crewItemName(path)+" "+path), strings.ToLower(c.filter)) {
			continue
		}
		o := ws.project.Dme.Objects[path]
		sprite := dmicon.Cache.GetSpriteOrPlaceholder(o.Vars.TextV("icon", ""), o.Vars.TextV("icon_state", ""))
		imgui.ImageV(imgui.TextureID(sprite.Texture()), imgui.Vec2{X: 32 * window.PointSize(), Y: 32 * window.PointSize()}, imgui.Vec2{X: sprite.U1, Y: sprite.V1}, imgui.Vec2{X: sprite.U2, Y: sprite.V2}, imgui.Vec4{X: 1, Y: 1, Z: 1, W: 1}, imgui.Vec4{})
		imgui.SameLine()
		imgui.BeginGroup()
		if imgui.SelectableV(ws.crewItemName(path)+"##"+path, false, 0, imgui.Vec2{}) {
			ws.chooseCrewItem(path)
		}
		tooltip(path)
		imgui.TextDisabled(strings.TrimPrefix(path, "/obj/item/"))
		imgui.EndGroup()
		count++
		if count == 200 {
			hint("Type more to narrow the results.")
			break
		}
	}
	if count == 0 {
		hint("No matching items for this slot.")
	}
	imgui.EndChild()
}
