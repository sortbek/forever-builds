// Builds dist/data from the engine data, the UI data checkout and the wago tables.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type dbJSON struct {
	Items []struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		Icon           string `json:"icon"`
		Type           int    `json:"type"`
		Quality        int    `json:"quality"`
		ReqLvl         int    `json:"reqLvl"`
		ScalingOptions map[string]struct {
			Ilvl int `json:"ilvl"`
		} `json:"scalingOptions"`
		RandomSuffixOptions []int `json:"randomSuffixOptions"`
	} `json:"items"`
	Enchants []struct {
		EffectID int    `json:"effectId"`
		Name     string `json:"name"`
	} `json:"enchants"`
	SpellIcons []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Icon string `json:"icon"`
	} `json:"spellIcons"`
}

func write(dir, name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		log.Fatal(err)
	}
}

func main() {
	engine := flag.String("engine", "engine", "sim engine checkout")
	uidata := flag.String("uidata", "uidata", "UI data checkout")
	wago := flag.String("wago", "wago", "wago CSV dir")
	att := flag.String("att", "att", "AllTheThings checkout (item sources)")
	out := flag.String("out", "dist/data", "output dir")
	commit := flag.String("commit", "unknown", "engine commit")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(*uidata, "assets/database/db.json"))
	if err != nil {
		log.Fatal(err)
	}
	var db dbJSON
	if err := json.Unmarshal(raw, &db); err != nil {
		log.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(*uidata, "assets/database/leftover_db.json"))
	if err != nil {
		log.Fatal(err)
	}
	var leftovers dbJSON
	if err := json.Unmarshal(raw, &leftovers); err != nil {
		log.Fatal(err)
	}
	// The required level for the items wago's ItemSparse lacks, which is about half of them.
	uiReqLevels := map[int]int{}
	for _, it := range append(leftovers.Items, db.Items...) {
		uiReqLevels[it.ID] = it.ReqLvl
	}
	iconIDs, err := loadIconFileIDs(*wago)
	if err != nil {
		log.Fatal(err)
	}
	qualities, reqLevels, err := loadQualities(*wago)
	if err != nil {
		log.Fatal(err)
	}
	// The items the engine sims, shown with the UI database's icon and quality where it has them.
	items, err := engineItems(filepath.Join(*engine, "data"))
	if err != nil {
		log.Fatal(err)
	}
	gear := make(map[int]bool, len(items))
	for id := range items {
		gear[id] = true
	}
	for _, it := range db.Items {
		row := items[it.ID]
		if row == nil {
			row = &item{ID: it.ID, Name: it.Name, Type: it.Type, Ilvl: it.ScalingOptions["0"].Ilvl}
			items[it.ID] = row
		}
		row.Icon, row.Quality = it.Icon, it.Quality
		row.Suffix = row.Suffix || len(it.RandomSuffixOptions) > 0
	}
	list := make([]*item, 0, len(items))
	for id, row := range items {
		if row.Icon == "" && iconIDs[id] > 0 {
			row.Icon = strconv.Itoa(iconIDs[id]) // a FileDataID; the icon CDN serves those by number
		}
		if row.Quality == 0 {
			row.Quality = qualities[id]
		}
		if level, ok := reqLevels[id]; ok {
			row.ReqLevel = level
		} else {
			row.ReqLevel = uiReqLevels[id]
		}
		list = append(list, row)
	}
	sort.Slice(list, func(a, b int) bool { return list[a].ID < list[b].ID })
	write(*out, "items.json", list)
	enchants := make([]named, 0, len(db.Enchants))
	for _, e := range db.Enchants {
		enchants = append(enchants, named{ID: e.EffectID, Name: e.Name})
	}
	write(*out, "enchants.json", enchants)
	icons := make([]named, 0, len(db.SpellIcons))
	spellIcons := map[int]string{}
	for _, s := range db.SpellIcons {
		icons = append(icons, named{ID: s.ID, Name: s.Name})
		spellIcons[s.ID] = s.Icon
	}
	spellNames, err := loadSpellNames(filepath.Join(*wago, "SpellName.csv"))
	if err != nil {
		log.Fatal(err)
	}
	used, err := engineSpellIDs(filepath.Join(*uidata, "sim"))
	if err != nil {
		log.Fatal(err)
	}
	engineNames, err := engineSpellNames(filepath.Join(*engine, "data/spells.jsonl"))
	if err != nil {
		log.Fatal(err)
	}
	for id, name := range engineNames {
		used[id] = true
		if spellNames[id] == "" {
			spellNames[id] = name
		}
	}
	spells := spellList(icons, spellNames, used)
	spellIconIDs, err := loadSpellIconIDs(*wago)
	if err != nil {
		log.Fatal(err)
	}
	for i := range spells {
		id := spells[i].ID
		if spellIcons[id] != "" {
			spells[i].Icon = spellIcons[id]
		} else if spellIconIDs[id] > 0 {
			spells[i].Icon = strconv.Itoa(spellIconIDs[id]) // a FileDataID
		}
	}
	write(*out, "spells.json", spells)
	tables, err := loadTraitTables(*wago)
	if err != nil {
		log.Fatal(err)
	}
	talents := map[string]*ClassTalents{}
	for class := range classIDs {
		ct, err := buildClassTalents(class, filepath.Join(*uidata, "ui/sim/talents/trees", class+".json"), tables, spellIcons)
		if err != nil {
			log.Fatal(err)
		}
		talents[class] = ct
	}
	write(*out, "talents.json", talents)
	mapNames, err := loadNames(filepath.Join(*wago, "UiMap.csv"))
	if err != nil {
		log.Fatal(err)
	}
	factionNames, err := loadNames(filepath.Join(*wago, "Faction.csv"))
	if err != nil {
		log.Fatal(err)
	}
	sources, err := buildSources(*att, gear, mapNames, factionNames)
	if err != nil {
		log.Fatal(err)
	}
	write(*out, "sources.json", sources)
	build, _ := os.ReadFile(filepath.Join(*wago, "BUILD"))
	write(*out, "version.json", map[string]string{"engine_commit": *commit, "client_build": strings.TrimSpace(string(build))})
	log.Printf("wrote %d items, %d enchants, %d spells, %d classes, %d item sources to %s", len(list), len(enchants), len(spells), len(talents), len(sources), *out)
}
