package wsplanet

func (w *Workspace) discardBiomeChanges() {
	// The name may be invalid or incomplete: discarding must not try to save it.
	w.nameDirty = false
	if !w.project.CanDiscardBiome(w.selected) {
		w.biomeName = w.project.State.Biomes[w.selected].Name
		w.message = "Unfinished biome name discarded."
		return
	}
	w.record()
	path, err := w.project.DiscardBiomeChanges(w.selected)
	if err != nil {
		w.message = err.Error()
		return
	}
	w.recordNamed("Discard biome changes")
	w.selectBiome(path)
	w.renderKey = ""
	w.message = "Biome changes discarded. Undo restores your edits."
}
