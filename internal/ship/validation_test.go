package ship

import "testing"

func TestCrewIdentifiersCannotCollide(t *testing.T) {
	c, env := authorEnvironment(t)
	p, e := NewProject(c, env, "crew_ids", "Crew IDs", 24, 24)
	if e != nil {
		t.Fatal(e)
	}
	crewTestTypes(p)
	jobs, _ := p.CrewJobs("ship")
	jobs[1].ID = "job_1"
	if e = p.SetCrewJobs("ship", jobs); e != nil {
		t.Fatal(e)
	}
	jobs, _ = p.CrewJobs("ship")
	if jobs[0].ID == jobs[1].ID {
		t.Fatal("auto-assigned ID collided with later explicit ID")
	}
	if e = p.SetCrewJobs("theme/standard", jobs); e == nil {
		t.Fatal("accepted IDs belonging to another roster")
	}
	jobs[0].ID = jobs[1].ID
	if e = p.SetCrewJobs("ship", jobs); e == nil {
		t.Fatal("accepted duplicate IDs in one roster")
	}
}

func TestThemeCloneRejectsInvalidSelection(t *testing.T) {
	c, env := authorEnvironment(t)
	p, e := NewProject(c, env, "theme_bounds", "Theme Bounds", 24, 24)
	if e != nil {
		t.Fatal(e)
	}
	before := p.settingsBytes()
	for _, index := range []int{-1, len(p.Hull.Themes)} {
		if e = p.AddTheme(index, "other", "Other", true); e == nil {
			t.Fatal("invalid theme accepted")
		}
	}
	if string(before) != string(p.settingsBytes()) {
		t.Fatal("failed clone changed settings")
	}
}
