package ship

import (
	"bytes"
	"os"
	"testing"
)

func TestVariantOnlyCrewRegistrationAndLegacyUpgrade(t *testing.T) {
	catalog, environment := authorEnvironment(t)
	project, err := NewProject(catalog, environment, "variant_crew", "Variant Crew", 24, 24)
	if err != nil {
		t.Fatal(err)
	}
	crewTestTypes(project)
	jobs, err := project.CrewJobs("ship")
	if err != nil {
		t.Fatal(err)
	}
	if err = project.SetCrewJobs("ship", nil); err != nil {
		t.Fatal(err)
	}
	if err = project.SetCrewJobs("theme/standard", jobs); err != nil {
		t.Fatal(err)
	}
	if err = project.AddTheme(0, "medical", "Medical", true); err != nil {
		t.Fatal(err)
	}
	if err = project.Save(); err != nil {
		t.Fatal(err)
	}
	path := project.outputPaths()[1]
	saved, _ := os.ReadFile(path)
	if !bytes.Contains(saved, []byte(`available_themes = list("standard", "medical")`)) || !bytes.Contains(saved, []byte("job_slots = list()")) {
		t.Fatal("hull did not register variants while keeping its base crew empty")
	}
	modules, _ := os.ReadFile(project.outputPaths()[2])
	if bytes.Count(modules, []byte("job_slots = list(")) != 2 {
		t.Fatal("variant rosters were lost")
	}
	legacy := bytes.Replace(saved, []byte(project.availableThemesLine()), nil, 1)
	if err = os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenProject(catalog, environment, project.Hull)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Modified() {
		t.Fatal("legacy hull was not queued for the theme registration repair")
	}
	if err = reopened.Save(); err != nil || reopened.Modified() {
		t.Fatalf("legacy hull was not upgraded cleanly: %v", err)
	}
	repaired, _ := os.ReadFile(path)
	if !bytes.Equal(saved, repaired) {
		t.Fatal("legacy upgrade changed more than the missing theme list")
	}
	// A legacy-looking hull with custom code must still be protected.
	custom := append(append([]byte{}, legacy...), []byte("\n// custom behavior\n")...)
	if err = os.WriteFile(path, custom, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err = OpenProject(catalog, environment, project.Hull)
	if err != nil {
		t.Fatal(err)
	}
	reopened.Settings.Description = "Updated description"
	if err = reopened.Save(); err == nil {
		t.Fatal("legacy migration overwrote custom code")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(custom, after) {
		t.Fatal("custom source changed")
	}
}
