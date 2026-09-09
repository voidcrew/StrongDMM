package mappreview

import (
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
)

// CompiledAppearances preserves all map overrides, including staged edits, and
// substitutes only their initial definitions. The source keeps editor sprites.
func CompiledAppearances(source *dmmap.Dmm, editor, compiled *dmenv.Dme) *dmmap.Dmm {
	result := source.Copy()
	cache := map[*dmmprefab.Prefab]*dmmprefab.Prefab{}
	for _, tile := range result.Tiles {
		for _, instance := range tile.Instances() {
			old := instance.Prefab()
			if cached := cache[old]; cached != nil {
				instance.SetPrefab(cached)
				continue
			}
			initial, replacement := editor.Objects[old.Path()], compiled.Objects[old.Path()]
			if initial == nil || replacement == nil {
				continue
			}
			var overrides []*dmvars.Variables
			for v := old.Vars(); v != nil && v != initial.Vars; v = v.Parent() {
				overrides = append(overrides, v)
			}
			vars := dmvars.FromParent(replacement.Vars)
			for i := len(overrides) - 1; i >= 0; i-- {
				for _, name := range overrides[i].Iterate() {
					value, _ := overrides[i].Value(name)
					vars = dmvars.Set(vars, name, value)
				}
			}
			updated := dmmprefab.New(dmmprefab.IdStage, old.Path(), vars)
			cache[old] = updated
			instance.SetPrefab(updated)
		}
	}
	return &result
}
