package main

import (
	"slices"
	"testing"
)

func TestPaladinTreeMatchesTraitTree1100(t *testing.T) {
	tables, err := loadTraitTables("testdata")
	if err != nil {
		t.Fatal(err)
	}
	ct, err := buildClassTalents("paladin", "testdata/paladin.json", tables)
	if err != nil {
		t.Fatal(err)
	}
	if ct.TreeID != 1100 || len(ct.Nodes) != 50 || len(ct.TreeSizes) != 3 {
		t.Errorf("tree %d, %d nodes, sizes %v", ct.TreeID, len(ct.Nodes), ct.TreeSizes)
	}
	for i := 1; i < len(ct.Nodes); i++ {
		if ct.Nodes[i-1].ID >= ct.Nodes[i].ID {
			t.Fatal("nodes not in ascending id order")
		}
	}
	if pos, ok := ct.Talents["135151"]; !ok || pos != [2]int{1, 1} {
		t.Errorf("Redoubt (135151) at %v, want [1 1]", pos)
	}
	if !slices.Contains(ct.SpecIDs, 1486) {
		t.Errorf("spec ids %v lack 1486", ct.SpecIDs)
	}
}
