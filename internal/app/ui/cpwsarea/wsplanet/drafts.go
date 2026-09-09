package wsplanet

import (
	"sort"
	"strings"

	"sdmm/internal/app/ui/workshop"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/planet"
)

type planetDraft struct {
	project                       *planet.Project
	selected, biomeName, entry    string
	nameDirty, cave, narrowEditor bool
	mode, table, settingsTab      int
}

// PreviewStates supplies snapshots to other workshop views without saving them.
func (w *Workspace) PreviewStates() []planet.State {
	var result []planet.State
	for _, draft := range w.drafts {
		result = append(result, planet.Clone(draft.project.State))
	}
	return result
}

func (w *Workspace) rememberCurrent() {
	if w.project == nil {
		return
	}
	w.record()
	if w.drafts == nil {
		w.drafts = map[string]*planetDraft{}
	}
	w.drafts[w.project.State.Definition.Path] = &planetDraft{
		project: w.project, selected: w.selected, biomeName: w.biomeName, entry: w.selectedEntry,
		nameDirty: w.nameDirty, cave: w.cave, narrowEditor: w.narrowEditor, mode: w.mode, table: w.table, settingsTab: w.settingsTab,
	}
}

func (w *Workspace) switchPlanet(path string) {
	if w.project != nil && path == w.project.State.Definition.Path {
		w.creating, w.cancelling = false, false
		return
	}
	draft := w.drafts[path]
	if draft == nil {
		for _, d := range w.catalog.Planets {
			if d.Path != path {
				continue
			}
			p, err := planet.Open(w.catalog, d)
			if err != nil {
				w.message = err.Error()
				return
			}
			w.rememberCurrent()
			if w.project != nil {
				w.previousPlanet = w.project.State.Definition.Path
			}
			w.bind(p)
			w.creating, w.cancelling = false, false
			w.message = ""
			return
		}
		return
	}
	w.rememberCurrent()
	if w.project != nil {
		w.previousPlanet = w.project.State.Definition.Path
	}
	w.project = draft.project
	w.last = planet.Clone(draft.project.State)
	w.selected, w.biomeName, w.selectedEntry = draft.selected, draft.biomeName, draft.entry
	w.nameDirty, w.cave, w.narrowEditor = draft.nameDirty, draft.cave, draft.narrowEditor
	w.mode, w.table = draft.mode, draft.table
	w.settingsTab = draft.settingsTab
	w.picking, w.deleting, w.review, w.creating, w.cancelling = false, false, false, false, false
	w.preview, w.scene = nil, nil
	w.renderKey, w.message = "", ""
	w.fit = true
	w.app.CommandStorage().SetStack(w.CommandStackId())
}

func (w *Workspace) planetChoices() []planet.Definition {
	byPath := map[string]planet.Definition{}
	for _, d := range w.catalog.Planets {
		byPath[d.Path] = d
	}
	for path, draft := range w.drafts {
		byPath[path] = draft.project.State.Definition
	}
	var result []planet.Definition
	for _, d := range byPath {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name+result[i].Path < result[j].Name+result[j].Path })
	return result
}

func (w *Workspace) startCreation() {
	w.rememberCurrent()
	w.creating, w.cancelling = true, false
	w.newName, w.message = "", ""
}

func (w *Workspace) createPlanet() {
	normalize := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	for _, draft := range w.drafts {
		if normalize(draft.project.State.Definition.Name) == normalize(w.newName) {
			w.message = "A planet already uses that name."
			return
		}
	}
	p, err := planet.Create(w.catalog, w.catalog.Planets[w.base], w.newName, w.blank)
	if err != nil {
		w.message = err.Error()
		return
	}
	if w.drafts[p.State.Definition.Path] != nil {
		w.message = "An open planet already uses that name's identifier."
		return
	}
	w.rememberCurrent()
	if w.project != nil {
		w.previousPlanet = w.project.State.Definition.Path
	}
	w.bind(p)
	w.creating, w.cancelling = false, false
	w.message = "New planet draft. Switch planets freely, or cancel it when you want to start over."
}

func (w *Workspace) discardNewPlanet() {
	if w.project == nil || !w.project.UnsavedNew() {
		return
	}
	path, name := w.project.State.Definition.Path, w.project.State.Definition.Name
	w.app.CommandStorage().DisposeStack(w.CommandStackId())
	delete(w.drafts, path)
	w.project = nil
	w.nameDirty, w.cancelling, w.review, w.creating = false, false, false, false
	w.preview, w.scene = nil, nil
	next := w.previousPlanet
	if next == path || w.drafts[next] == nil {
		next = ""
		if choices := w.planetChoices(); len(choices) > 0 {
			next = choices[0].Path
		}
	}
	w.switchPlanet(next)
	w.message = name + " discarded. No planet files were created."
}

func (w *Workspace) cancelPlanetForm() {
	workshop.Title("Discard new planet?")
	workshop.Muted(w.project.State.Definition.Name + " has not been saved. Discarding it removes this draft and its biome edits. Your other planets stay open.")
	workshop.Gap()
	if workshop.Button("Discard new planet", true) {
		w.discardNewPlanet()
	}
	if workshop.Button("Keep building", false) {
		w.cancelling = false
	}
}

func (w *Workspace) currentModified() bool {
	return w.project != nil && (w.nameDirty || w.project.Modified())
}

func (w *Workspace) saveCurrent() bool {
	if w.project == nil {
		return true
	}
	if !w.commitName() {
		return false
	}
	w.record()
	changes, err := w.project.Changes()
	if err == nil {
		err = w.project.Save()
	}
	if err != nil {
		w.message = err.Error()
		return false
	}
	if dme, err := dmenv.New(w.catalog.Dme.RootFile); err == nil {
		if catalog, err := planet.Discover(dme); err == nil {
			w.catalog = catalog
		}
	}
	w.project.Catalog = w.catalog
	for _, draft := range w.drafts {
		if draft.project != w.project {
			draft.project.ObserveSave(changes, w.catalog)
		}
	}
	w.last = planet.Clone(w.project.State)
	w.rememberCurrent()
	w.review = false
	w.message = "Planet saved to the project."
	return true
}

// The workspace's normal Save/close action covers every open draft. The review
// panel uses saveCurrent, matching the one planet whose files it displays.
func (w *Workspace) Save() bool {
	w.rememberCurrent()
	original := ""
	if w.project != nil {
		original = w.project.State.Definition.Path
	}
	for _, d := range w.planetChoices() {
		draft := w.drafts[d.Path]
		if draft == nil || (!draft.nameDirty && !draft.project.Modified()) {
			continue
		}
		w.switchPlanet(d.Path)
		if !w.saveCurrent() {
			return false
		}
	}
	w.switchPlanet(original)
	w.message = "All open planets saved to the project."
	return true
}
