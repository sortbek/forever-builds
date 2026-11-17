package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type item struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Icon     string `json:"icon"`
	Type     int    `json:"type"`
	Quality  int    `json:"quality"`
	Ilvl     int    `json:"ilvl"`
	ReqLevel int    `json:"req_level,omitempty"`
	Suffix   bool   `json:"suffix,omitempty"`
	// Who can wear it: cloth/leather/mail/plate, the weapon kind, the hand it is held in, and the
	// classes it is limited to.
	Armor   string   `json:"armor,omitempty"`
	Weapon  string   `json:"weapon,omitempty"`
	Hand    string   `json:"hand,omitempty"`
	Classes []string `json:"classes,omitempty"`
}

// proto.ItemType numbers, which SimHammer slots items by.
var itemTypes = map[string]int{
	"ItemTypeHead": 1, "ItemTypeNeck": 2, "ItemTypeShoulder": 3, "ItemTypeBack": 4, "ItemTypeChest": 5,
	"ItemTypeWrist": 6, "ItemTypeHands": 7, "ItemTypeWaist": 8, "ItemTypeLegs": 9, "ItemTypeFeet": 10,
	"ItemTypeFinger": 11, "ItemTypeTrinket": 12, "ItemTypeWeapon": 13, "ItemTypeRanged": 14,
}

type engineItem struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	ArmorType      string   `json:"armor_type"`
	WeaponType     string   `json:"weapon_type"`
	RangedType     string   `json:"ranged_weapon_type"`
	HandType       string   `json:"hand_type"`
	ClassAllowlist []string `json:"class_allowlist"`
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
			row := &item{ID: it.ID, Name: it.Name, Type: itemTypes[it.Type], Ilvl: it.ScalingOptions["0"].Ilvl}
			row.Armor = enumName(it.ArmorType, "ArmorType")
			row.Weapon = enumName(it.WeaponType, "WeaponType") + enumName(it.RangedType, "RangedWeaponType")
			row.Hand = enumName(it.HandType, "HandType")
			for _, c := range it.ClassAllowlist {
				row.Classes = append(row.Classes, enumName(c, "Class"))
			}
			items[it.ID] = row
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

// Each item's quality and required level.
// ArmorTypeLeather → leather, HandTypeTwoHand → two_hand; empty for an unset or unknown value.
func enumName(value, prefix string) string {
	name, ok := strings.CutPrefix(value, prefix)
	if !ok || name == "" || name == "Unknown" {
		return ""
	}
	var out []rune
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				out = append(out, '_')
			}
			r = unicode.ToLower(r)
		}
		out = append(out, r)
	}
	return string(out)
}

func loadQualities(dir string) (map[int]int, map[int]int, error) {
	rows, err := readCSV(filepath.Join(dir, "ItemSparse.csv"))
	if err != nil {
		return nil, nil, err
	}
	quality, level := make(map[int]int, len(rows)), make(map[int]int, len(rows))
	for _, r := range rows {
		quality[atoi(r["ID"])] = atoi(r["OverallQualityID"])
		level[atoi(r["ID"])] = atoi(r["RequiredLevel"])
	}
	return quality, level, nil
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
