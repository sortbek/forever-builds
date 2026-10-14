package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type NodeEntry struct {
	DefinitionID int `json:"definition_id"`
	MaxRanks     int `json:"max_ranks"`
}

type TalentNode struct {
	ID      int         `json:"id"`
	Entries []NodeEntry `json:"entries"`
}

type ClassTalents struct {
	TreeID    int               `json:"tree_id"`
	SpecIDs   []int             `json:"spec_ids"`
	Nodes     []TalentNode      `json:"nodes"`
	Talents   map[string][2]int `json:"talents"`
	TreeSizes []int             `json:"tree_sizes"`
	Trees     []TreeLayout      `json:"trees"`
}

// The talent calculator layout, in the engine's talent-string order.
type TreeLayout struct {
	Name    string         `json:"name"`
	Talents []TalentLayout `json:"talents"`
}

type TalentLayout struct {
	Name     string  `json:"name"`
	SpellID  int     `json:"spell_id"`
	Icon     string  `json:"icon"`
	Row      int     `json:"row"`
	Col      int     `json:"col"`
	MaxRanks int     `json:"max_ranks"`
	Prereq   *[2]int `json:"prereq,omitempty"` // [row, col] of the talent it needs
}

// ChrClasses ids, as ChrSpecialization.ClassID uses them.
var classIDs = map[string]int{
	"warrior": 1, "paladin": 2, "hunter": 3, "rogue": 4, "priest": 5,
	"shaman": 7, "mage": 8, "warlock": 9, "druid": 11,
}

type traitTables struct {
	nodesByTree   map[int][]int
	entriesByNode map[int][][2]int // node id -> [(index, entry id)]
	entries       map[int]NodeEntry
	specsByClass  map[int][]int
}

func readCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	var out []map[string]string
	for _, r := range rows[1:] {
		m := map[string]string{}
		for i, h := range rows[0] {
			if i < len(r) {
				m[h] = r[i]
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func loadTraitTables(dir string) (*traitTables, error) {
	t := &traitTables{nodesByTree: map[int][]int{}, entriesByNode: map[int][][2]int{}, entries: map[int]NodeEntry{}, specsByClass: map[int][]int{}}
	nodes, err := readCSV(filepath.Join(dir, "TraitNode.csv"))
	if err != nil {
		return nil, err
	}
	for _, r := range nodes {
		tree := atoi(r["TraitTreeID"])
		t.nodesByTree[tree] = append(t.nodesByTree[tree], atoi(r["ID"]))
	}
	links, err := readCSV(filepath.Join(dir, "TraitNodeXTraitNodeEntry.csv"))
	if err != nil {
		return nil, err
	}
	for _, r := range links {
		n := atoi(r["TraitNodeID"])
		t.entriesByNode[n] = append(t.entriesByNode[n], [2]int{atoi(r["_Index"]), atoi(r["TraitNodeEntryID"])})
	}
	entries, err := readCSV(filepath.Join(dir, "TraitNodeEntry.csv"))
	if err != nil {
		return nil, err
	}
	for _, r := range entries {
		t.entries[atoi(r["ID"])] = NodeEntry{DefinitionID: atoi(r["TraitDefinitionID"]), MaxRanks: atoi(r["MaxRanks"])}
	}
	specs, err := readCSV(filepath.Join(dir, "ChrSpecialization.csv"))
	if err != nil {
		return nil, err
	}
	for _, r := range specs {
		class := atoi(r["ClassID"])
		t.specsByClass[class] = append(t.specsByClass[class], atoi(r["ID"]))
	}
	return t, nil
}

type engineLocation struct {
	Row int `json:"rowIdx"`
	Col int `json:"colIdx"`
}

type engineTree struct {
	Name    string `json:"name"`
	Talents []struct {
		DefinitionID int             `json:"definitionId"`
		Name         string          `json:"fancyName"`
		SpellID      int             `json:"spellId"`
		MaxPoints    int             `json:"maxPoints"`
		Location     engineLocation  `json:"location"`
		Prereq       *engineLocation `json:"prereqLocation"`
	} `json:"talents"`
}

// The class's trait tree is the one whose entries carry the most of the engine tree's definition ids.
func buildClassTalents(class, treeJSON string, t *traitTables, spellIcons map[int]string) (*ClassTalents, error) {
	data, err := os.ReadFile(treeJSON)
	if err != nil {
		return nil, err
	}
	var trees []engineTree
	if err := json.Unmarshal(data, &trees); err != nil {
		return nil, err
	}
	ct := &ClassTalents{Talents: map[string][2]int{}, SpecIDs: t.specsByClass[classIDs[class]]}
	sort.Ints(ct.SpecIDs)
	defs := map[int]bool{}
	for ti, tree := range trees {
		ct.TreeSizes = append(ct.TreeSizes, len(tree.Talents))
		layout := TreeLayout{Name: tree.Name}
		for i, tal := range tree.Talents {
			if tal.DefinitionID != 0 {
				ct.Talents[strconv.Itoa(tal.DefinitionID)] = [2]int{ti, i}
				defs[tal.DefinitionID] = true
			}
			tl := TalentLayout{Name: tal.Name, SpellID: tal.SpellID, Icon: spellIcons[tal.SpellID],
				Row: tal.Location.Row, Col: tal.Location.Col, MaxRanks: tal.MaxPoints}
			if tal.Prereq != nil {
				tl.Prereq = &[2]int{tal.Prereq.Row, tal.Prereq.Col}
			}
			layout.Talents = append(layout.Talents, tl)
		}
		ct.Trees = append(ct.Trees, layout)
	}
	bestTree, bestHits := 0, 0
	for tree, nodeIDs := range t.nodesByTree {
		hits := 0
		for _, n := range nodeIDs {
			for _, link := range t.entriesByNode[n] {
				if defs[t.entries[link[1]].DefinitionID] {
					hits++
				}
			}
		}
		if hits > bestHits {
			bestTree, bestHits = tree, hits
		}
	}
	if bestHits == 0 {
		return nil, fmt.Errorf("%s: no trait tree matches", treeJSON)
	}
	ct.TreeID = bestTree
	nodeIDs := append([]int(nil), t.nodesByTree[bestTree]...)
	sort.Ints(nodeIDs)
	for _, n := range nodeIDs {
		links := append([][2]int(nil), t.entriesByNode[n]...)
		sort.Slice(links, func(a, b int) bool { return links[a][0] < links[b][0] })
		node := TalentNode{ID: n}
		for _, l := range links {
			node.Entries = append(node.Entries, t.entries[l[1]])
		}
		ct.Nodes = append(ct.Nodes, node)
	}
	return ct, nil
}
