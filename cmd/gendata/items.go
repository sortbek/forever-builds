package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

type item struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Icon    string `json:"icon"`
	Type    int    `json:"type"`
	Quality int    `json:"quality"`
	Ilvl    int    `json:"ilvl"`
	Suffix  bool   `json:"suffix,omitempty"`
}

// proto.ItemType numbers, which SimHammer slots items by.
var itemTypes = map[string]int{
	"ItemTypeHead": 1, "ItemTypeNeck": 2, "ItemTypeShoulder": 3, "ItemTypeBack": 4, "ItemTypeChest": 5,
	"ItemTypeWrist": 6, "ItemTypeHands": 7, "ItemTypeWaist": 8, "ItemTypeLegs": 9, "ItemTypeFeet": 10,
	"ItemTypeFinger": 11, "ItemTypeTrinket": 12, "ItemTypeWeapon": 13, "ItemTypeRanged": 14,
}

type engineItem struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	ScalingOptions map[string]struct {
		Ilvl int `json:"ilvl"`
	} `json:"scaling_options"`
}

func readLines(path string, each func([]byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<26)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		if err := each(sc.Bytes()); err != nil {
			return err
		}
	}
	return sc.Err()
}

// Every item the engine has data for; the later files are the live game's rows of the same items.
func engineItems(dir string) (map[int]*item, error) {
	items := map[int]*item{}
	for _, name := range []string{"items.jsonl", "forever-client-items.jsonl", "forever-live-items.jsonl"} {
		err := readLines(filepath.Join(dir, name), func(line []byte) error {
			var it engineItem
			if err := json.Unmarshal(line, &it); err != nil {
				return err
			}
			if itemTypes[it.Type] == 0 {
				return nil
			}
			items[it.ID] = &item{ID: it.ID, Name: it.Name, Type: itemTypes[it.Type], Ilvl: it.ScalingOptions["0"].Ilvl}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	err := readLines(filepath.Join(dir, "forever-random-enchantment-items.jsonl"), func(line []byte) error {
		var row struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			return err
		}
		if it := items[row.ID]; it != nil {
			it.Suffix = true
		}
		return nil
	})
	return items, err
}

func loadQualities(dir string) (map[int]int, error) {
	rows, err := readCSV(filepath.Join(dir, "ItemSparse.csv"))
	if err != nil {
		return nil, err
	}
	out := make(map[int]int, len(rows))
	for _, r := range rows {
		out[atoi(r["ID"])] = atoi(r["OverallQualityID"])
	}
	return out, nil
}

func engineSpellNames(path string) (map[int]string, error) {
	names := map[int]string{}
	err := readLines(path, func(line []byte) error {
		var row struct {
			ID   int    `json:"ID"`
			Name string `json:"Name"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			return err
		}
		names[row.ID] = row.Name
		return nil
	})
	return names, err
}
