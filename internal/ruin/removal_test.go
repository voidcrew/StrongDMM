package ruin

import (
	"os"
	"path/filepath"
	"sdmm/internal/ship"
	"strings"
	"testing"
)

func TestRuinEditsSurviveShipRemoval(t *testing.T) {
	c := environment(t)
	s := setup(c)
	p, err := New(c, s)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	shipCode := filepath.Join(c.Dme.RootDir, "ship.dm")
	write(t, shipCode, []byte(ship.HullType+"/removed\n\tvar/name = \"Removed\"\n"))
	write(t, c.Dme.RootFile, append(read(t, c.Dme.RootFile), []byte("#include \"ship.dm\"\n")...))
	c = reload(t, c)
	p, err = Open(c, find(t, c, p.Template.Type))
	if err != nil {
		t.Fatal(err)
	}
	p.Properties.Name = "Unsaved property edit"
	plan, err := ship.PlanRemoval(c.Dme, ship.RemovalSpec{Name: "Removed", Types: []string{ship.HullType + "/removed"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	c.Dme.RemoveTypeTrees(plan.Types)
	p.RebaseRemoval(plan.Changes)
	if err := p.Save(); err != nil {
		t.Fatal("other workshop lost its pending edits", err)
	}
	c = reload(t, c)
	if find(t, c, p.Template.Type).Name != p.Properties.Name {
		t.Fatal("property edit did not persist")
	}
	if c.Dme.Objects[ship.HullType+"/removed"] != nil {
		t.Fatal("saving resurrected deleted ship")
	}
}

func TestRemoveRuinRoundTrip(t *testing.T) {
	for _, anywhere := range []bool{false, true} {
		c := environment(t)
		s := setup(c)
		s.Anywhere = anywhere
		if anywhere {
			s.Area = "/area/ruin/unpowered"
		}
		p, err := New(c, s)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Save(); err != nil {
			t.Fatal(err)
		}
		plan, err := c.Removal(p.Template)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Apply(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(p.Template.File); !os.IsNotExist(err) {
			t.Fatal("ruin map not removed", err)
		}
		fresh := reload(t, c)
		for _, entry := range fresh.Templates {
			if entry.ID == s.ID {
				t.Fatal("removed ruin still registered")
			}
		}
		if _, err := New(fresh, s); err != nil {
			t.Fatal("removed ruin identifier is still reserved", err)
		}
	}
}

func TestRemoveHandwrittenRuinPreservesNeighbors(t *testing.T) {
	c := environment(t)
	file := filepath.Join(c.Dme.RootDir, "_maps/voidcrew/RandomRuins/SpaceRuins/old.dmm")
	write(t, file, []byte("shared map bytes"))
	// Removing the parent also removes its linked subtype and custom procedure.
	plan, err := c.Removal(find(t, c, Type+"/space/old"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	data := string(read(t, filepath.Join(c.Dme.RootDir, "fixture.dm")))
	if strings.Contains(data, Type+"/space/old") || !strings.Contains(data, Type+"/icemoon/underground") {
		t.Fatal("wrong registration removed")
	}
	fresh := reload(t, c)
	if fresh.Dme.Objects[Type+"/space/old"] != nil {
		t.Fatal("removed definition still parses")
	}
}

func TestRemoveLinkedChildRequiresResolvingDependency(t *testing.T) {
	c := environment(t)
	if _, err := c.Removal(find(t, c, Type+"/space/old/linked")); err == nil {
		t.Fatal("removed a ruin still in another ruin's spawn list")
	}
}

func TestRemoveRuinKeepsSharedMap(t *testing.T) {
	c := environment(t)
	s := setup(c)
	p, err := New(c, s)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(c.Dme.RootDir, "fixture.dm")
	write(t, source, append(read(t, source), []byte("\n"+Type+"/space/another\n\tid = \"another\"\n\tname = \"Another ruin\"\n\tsuffix = \""+s.ID+".dmm\"\n")...))
	c = reload(t, c)
	plan, err := c.Removal(find(t, c, p.Template.Type))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Template.File); err != nil {
		t.Fatal("shared map was deleted", err)
	}
	c = reload(t, c)
	if c.Dme.Objects[Type+"/space/another"] == nil {
		t.Fatal("removed unrelated ruin")
	}
}
