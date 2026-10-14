// Builds dist/data from the engine checkout and the wago tables.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
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
	engine := flag.String("engine", "engine", "engine checkout")
	wago := flag.String("wago", "wago", "wago CSV dir")
	out := flag.String("out", "dist/data", "output dir")
	commit := flag.String("commit", "unknown", "engine commit")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(*engine, "assets/database/db.json"))
	if err != nil {
		log.Fatal(err)
	}
	var db dbJSON
	if err := json.Unmarshal(raw, &db); err != nil {
		log.Fatal(err)
	}
	type item struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Icon    string `json:"icon"`
		Type    int    `json:"type"`
		Quality int    `json:"quality"`
		Ilvl    int    `json:"ilvl"`
		Suffix  bool   `json:"suffix,omitempty"`
	}
	iconIDs, err := loadIconFileIDs(*wago)
	if err != nil {
		log.Fatal(err)
	}
	items := make([]item, 0, len(db.Items))
	for _, it := range db.Items {
		icon := it.Icon
		if icon == "" && iconIDs[it.ID] > 0 {
			icon = strconv.Itoa(iconIDs[it.ID]) // a FileDataID; the icon CDN serves those by number
		}
		items = append(items, item{it.ID, it.Name, icon, it.Type, it.Quality, it.ScalingOptions["0"].Ilvl, len(it.RandomSuffixOptions) > 0})
	}
	write(*out, "items.json", items)
	enchants := make([]named, 0, len(db.Enchants))
	for _, e := range db.Enchants {
		enchants = append(enchants, named{e.EffectID, e.Name})
	}
	write(*out, "enchants.json", enchants)
	icons := make([]named, 0, len(db.SpellIcons))
	for _, s := range db.SpellIcons {
		icons = append(icons, named{s.ID, s.Name})
	}
	spellNames, err := loadSpellNames(filepath.Join(*wago, "SpellName.csv"))
	if err != nil {
		log.Fatal(err)
	}
	used, err := engineSpellIDs(filepath.Join(*engine, "sim"))
	if err != nil {
		log.Fatal(err)
	}
	spells := spellList(icons, spellNames, used)
	write(*out, "spells.json", spells)
	tables, err := loadTraitTables(*wago)
	if err != nil {
		log.Fatal(err)
	}
	talents := map[string]*ClassTalents{}
	for class := range classIDs {
		ct, err := buildClassTalents(class, filepath.Join(*engine, "ui/sim/talents/trees", class+".json"), tables)
		if err != nil {
			log.Fatal(err)
		}
		talents[class] = ct
	}
	write(*out, "talents.json", talents)
	build, _ := os.ReadFile(filepath.Join(*wago, "BUILD"))
	write(*out, "version.json", map[string]string{"engine_commit": *commit, "client_build": strings.TrimSpace(string(build))})
	log.Printf("wrote %d items, %d enchants, %d spells, %d classes to %s", len(items), len(enchants), len(spells), len(talents), *out)
}
