package ship

import (
	"bytes"
	"strings"
	"testing"
)

func TestRewriteRoomSlotsPreservesOtherDefinitions(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		prefix := "/* /datum/ship_theme/sample\n\tupgrade_slot_ids = list(\"ignore\") */\n/datum/ship_theme/sample\n\tdesc = \"list(fake) // text\"\n\tupgrade_slot_ids = "
		expression := "list(\n\t\t\"old\", // old room\n\t)"
		suffix := " // Keep this comment\n\tjob_slots = list(list(name = \"Engineer\", slots = 2))\n\tpart_cost = list(PART_CLASS_TRADE = 17)\n\n/datum/ship_theme/sample/other\n\tupgrade_slot_ids = list(\"old\")\n"
		original := strings.ReplaceAll(prefix+expression+suffix, "\n", newline)
		got, err := rewriteRoomSlots([]byte(original), "/datum/ship_theme/sample", []string{"old"}, []string{"old", "new_room"})
		if err != nil {
			t.Fatal(err)
		}
		want := strings.ReplaceAll(prefix+`list("old", "new_room")`+suffix, "\n", newline)
		if string(got) != want {
			t.Fatalf("unexpected source change: %s", got)
		}
		if _, err = rewriteRoomSlots([]byte(original), "/datum/ship_theme/sample", []string{"stale"}, []string{"stale", "new_room"}); err == nil {
			t.Fatal("accepted stale environment")
		}
	}
}

func TestRewriteInheritedAndUnsupportedRoomLists(t *testing.T) {
	for _, source := range []string{"/datum/ship_theme/sample", "/datum/ship_theme/sample\n\tname = \"Sample\"\n"} {
		got, err := rewriteRoomSlots([]byte(source), "/datum/ship_theme/sample", nil, []string{"base", "new"})
		if err != nil || !bytes.Contains(got, []byte("\n\tupgrade_slot_ids = list(\"base\", \"new\")\n")) {
			t.Fatalf("could not add inherited list: %v %s", err, got)
		}
	}
	for _, value := range []string{"DEFAULT_SLOTS", "list(\"old\") + list(\"other\")", "nullified", "list(\"old\""} {
		_, err := rewriteRoomSlots([]byte("/datum/ship_theme/sample\n\tupgrade_slot_ids = "+value+"\n"), "/datum/ship_theme/sample", []string{"old"}, []string{"old", "new"})
		if err == nil {
			t.Fatal("accepted unsupported room list:", value)
		}
	}
}
