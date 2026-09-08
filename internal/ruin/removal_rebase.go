package ruin

import "sdmm/internal/ship"

func (p *Project) RebaseRemoval(changes []ship.FileChange) {
	p.source = ship.RebaseAfterRemoval(p.source, changes)
	p.dme = ship.RebaseAfterRemoval(p.dme, changes)
	for i, source := range p.dependencies {
		p.dependencies[i] = ship.RebaseAfterRemoval(source, changes)
	}
	for _, member := range p.companions {
		member.RebaseRemoval(changes)
	}
}
