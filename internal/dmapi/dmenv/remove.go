package dmenv

import "strings"

// RemoveTypeTrees updates the loaded environment only after source removal
// succeeds. Retained objects must have no references to these trees.
func (d *Dme) RemoveTypeTrees(roots []string) {
	removed := map[string]bool{}
	for path := range d.Objects {
		for _, root := range roots {
			if path == root || strings.HasPrefix(path, root+"/") {
				removed[path] = true
				delete(d.Objects, path)
				break
			}
		}
	}
	for _, obj := range d.Objects {
		children := obj.DirectChildren[:0]
		for _, path := range obj.DirectChildren {
			if !removed[path] {
				children = append(children, path)
			}
		}
		obj.DirectChildren = children
	}
}
