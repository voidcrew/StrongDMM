package dmmdata

import (
	"path/filepath"
	"testing"
)

func TestSaveReportsUnwritableDestination(t *testing.T) {
	for _, tgm := range []bool{false, true} {
		data := DmmData{IsTgm: tgm, LineBreak: "\n", Filepath: filepath.Join(t.TempDir(), "missing", "ship.dmm")}
		if err := data.Save(); err == nil {
			t.Fatal("failed map write reported success")
		}
	}
}
