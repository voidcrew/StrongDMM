package command

import "testing"

func TestCommitAfterFocusChangeKeepsSourceUndo(t *testing.T) {
	s := NewStorage()
	s.SetStack("source.dmm")
	s.SetStack(NullSpaceStackId) // The user moved to the assembled preview.
	value := 1
	s.PushV("source.dmm", Make("Source edit", func() { value = 0 }, func() { value = 1 }))
	if !s.IsModified("source.dmm") || s.HasUndo() {
		t.Fatal("command attached to the preview")
	}
	s.SetStack("source.dmm")
	s.Undo()
	if value != 0 {
		t.Fatal("source undo lost")
	}
	s.Redo()
	if value != 1 {
		t.Fatal("source redo lost")
	}
}
