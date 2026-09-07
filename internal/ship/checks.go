package ship

import "strings"

func (a *Assembly) CheckHull() {
	ports, pods, floors := 0, 0, 0
	for _, atoms := range a.Cells {
		for _, atom := range atoms {
			path := atom.Prefab.Path()
			if strings.HasPrefix(path, "/obj/docking_port/mobile/") {
				ports++
			}
			if atom.Source == 0 && strings.HasPrefix(path, "/obj/machinery/cryopod") {
				pods++
			}
			if strings.HasPrefix(path, "/turf/open/floor") {
				floors++
			}
		}
	}
	if ports != 1 {
		a.Issues = append(a.Issues, Issue{Message: "Hull needs exactly one mobile docking port."})
	}
	if pods == 0 {
		a.Issues = append(a.Issues, Issue{Message: "Add a permanent crew spawn pod to the hull."})
	}
	if floors == 0 {
		a.Issues = append(a.Issues, Issue{Message: "Lay out the permanent deck and assign ship areas."})
	}
}
