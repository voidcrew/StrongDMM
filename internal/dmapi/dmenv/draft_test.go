package dmenv

import "testing"

func TestInvalidDraftTypeReturnsError(t *testing.T) {
	d := &Dme{Objects: map[string]*Object{}}
	for _, path := range []string{"", "area", "/area", "area/room", "/area/"} {
		if d.AddDraftType(path, nil) == nil {
			t.Fatalf("accepted invalid draft path %q", path)
		}
	}
}
