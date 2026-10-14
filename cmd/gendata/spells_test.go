package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEngineSpellIDsFindsActionSpells(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pet.go"), []byte("a := core.ActionID{SpellID: 3009}\nb := ActionID{SpellID:17261, Tag: 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("SpellID: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := engineSpellIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[int]bool{3009: true, 17261: true}) {
		t.Errorf("got %v", got)
	}
}

func TestSpellListAddsClientNamesForEngineSpells(t *testing.T) {
	icons := []named{{1, "Icon Name"}}
	names := map[int]string{1: "Client Name", 3009: "Claw", 5: "Unused"}
	got := spellList(icons, names, map[int]bool{1: true, 3009: true})
	want := []named{{1, "Icon Name"}, {3009, "Claw"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}
