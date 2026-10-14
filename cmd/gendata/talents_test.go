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
	ct, err := buildClassTalents("paladin", "testdata/paladin.json", tables, nil)
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

func TestPaladinTreesCarryTheCalculatorLayout(t *testing.T) {
	tables, err := loadTraitTables("testdata")
	if err != nil {
		t.Fatal(err)
	}
	icons := map[int]string{20262: "ability_golemthunderclap"}
	ct, err := buildClassTalents("paladin", "testdata/paladin.json", tables, icons)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct.Trees) != 3 || ct.Trees[0].Name != "Holy" || ct.Trees[2].Name != "Retribution" {
		t.Fatalf("trees %+v", ct.Trees)
	}
	for i, tree := range ct.Trees {
		if len(tree.Talents) != ct.TreeSizes[i] {
			t.Errorf("tree %d has %d talents, size %d", i, len(tree.Talents), ct.TreeSizes[i])
		}
	}
	want := TalentLayout{Name: "Divine Strength", SpellID: 20262, Icon: "ability_golemthunderclap", Row: 0, Col: 1, MaxRanks: 5}
	if got := ct.Trees[0].Talents[0]; got != want {
		t.Errorf("first Holy talent %+v, want %+v", got, want)
	}
	pos := ct.Talents["135223"] // Vengeance, below Conviction
	if v := ct.Trees[pos[0]].Talents[pos[1]]; v.Name != "Vengeance" || v.Prereq == nil || *v.Prereq != [2]int{2, 1} {
		t.Errorf("Vengeance %+v", v)
	}
}
