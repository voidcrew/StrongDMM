package ship

import "bytes"

// Advance shared source snapshots after a confirmed removal elsewhere in the
// editor. Only an exact before-image can be rebased; external conflicts remain.
func RebaseAfterRemoval(source FileChange, changes []FileChange) FileChange {
	for _, change := range changes {
		if source.Path == change.Path && !change.Delete && source.Existed && bytes.Equal(source.Before, change.Before) {
			source.Before = append([]byte{}, change.After...)
			break
		}
	}
	return source
}

func (p *Project) RebaseRemoval(changes []FileChange) {
	for _, change := range changes {
		if bytes.Equal(p.costSources[change.Path], change.Before) {
			if change.Delete {
				delete(p.costSources, change.Path)
			} else if p.costSources != nil {
				p.costSources[change.Path] = append([]byte{}, change.After...)
			}
		}
	}
	for path, source := range p.files {
		if p.rooms != nil {
			if original, ok := p.rooms.sources[path]; ok {
				conflict := false
				for _, change := range changes {
					if change.Path == path && !bytes.Equal(original.Before, change.Before) {
						conflict = true
						break
					}
				}
				// Room undo regenerates from its original source. Do not advance
				// the conflict guard if that source cannot be rebased exactly.
				if conflict {
					continue
				}
			}
		}
		p.files[path] = RebaseAfterRemoval(source, changes)
	}
	if p.rooms != nil {
		for path, source := range p.rooms.sources {
			p.rooms.sources[path] = RebaseAfterRemoval(source, changes)
		}
	}
}
