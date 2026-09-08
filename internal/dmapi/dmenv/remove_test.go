package dmenv

import "testing"

func TestRemoveTypeTreesKeepsSimilarNames(t *testing.T) {
	d := &Dme{Objects: map[string]*Object{
		"/datum":            {Path: "/datum", DirectChildren: []string{"/datum/ship", "/datum/shipwreck"}},
		"/datum/ship":       {Path: "/datum/ship"},
		"/datum/ship/child": {Path: "/datum/ship/child"},
		"/datum/shipwreck":  {Path: "/datum/shipwreck"},
	}}
	d.RemoveTypeTrees([]string{"/datum/ship"})
	if len(d.Objects) != 2 || d.Objects["/datum/shipwreck"] == nil {
		t.Fatal("removed neighboring type")
	}
	children := d.Objects["/datum"].DirectChildren
	if len(children) != 1 || children[0] != "/datum/shipwreck" {
		t.Fatal("stale environment children", children)
	}
}
