package ship

import (
	"sdmm/internal/util"
	"testing"
)

func TestRoomCloneKeepsIndependentCrew(t *testing.T) {
	c, env := authorEnvironment(t)
	p, e := NewProject(c, env, "clone_crew", "Clone Crew", 24, 24)
	if e != nil {
		t.Fatal(e)
	}
	crewTestTypes(p)
	if e = p.addRect(0, "lab", "Lab", util.Point{X: 3, Y: 3, Z: 1}, util.Point{X: 5, Y: 5, Z: 1}); e != nil {
		t.Fatal(e)
	}
	jobs := []CrewJob{{Name: "Scientist", Slots: 2, Category: "Science", Outfit: "/datum/outfit/job/assistant", Equipment: map[string]string{"head": ""}}}
	if e = p.SetCrewJobs("module/lab_basic", jobs); e != nil {
		t.Fatal(e)
	}
	if e = p.AddModule(0, p.Hull.Modules[0], "science", "Science", false); e != nil {
		t.Fatal(e)
	}
	original, _ := p.CrewJobs("module/lab_basic")
	cloned, _ := p.CrewJobs("module/science")
	if len(cloned) != 1 || cloned[0].Name != "Scientist" || cloned[0].Slots != 2 || cloned[0].ID == original[0].ID || cloned[0].Outfit == original[0].Outfit {
		t.Fatalf("clone lost crew or shared its outfit: %+v", cloned)
	}
	if e = p.Save(); e != nil {
		t.Fatal(e)
	}
	reopened, e := OpenProject(c, env, p.Hull)
	if e != nil {
		t.Fatal(e)
	}
	saved, _ := reopened.CrewJobs("module/science")
	if len(saved) != 1 || saved[0].BaseOutfit != "/datum/outfit/job/assistant" {
		t.Fatalf("reopen lost cloned outfit: %+v", saved)
	}
	if e = p.AddModule(0, p.Hull.Modules[0], "empty", "Empty", true); e != nil {
		t.Fatal(e)
	}
	empty, _ := p.CrewJobs("module/empty")
	if len(empty) != 0 {
		t.Fatal("empty room unexpectedly copied crew")
	}
}
