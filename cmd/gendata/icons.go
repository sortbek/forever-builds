package main

import "path/filepath"

// Icon FileDataIDs from the client tables, for items db.json names no icon for (mostly new
// Forever items). Item.IconFileDataID first, else the base appearance's default icon.
func loadIconFileIDs(dir string) (map[int]int, error) {
	items, err := readCSV(filepath.Join(dir, "Item.csv"))
	if err != nil {
		return nil, err
	}
	links, err := readCSV(filepath.Join(dir, "ItemModifiedAppearance.csv"))
	if err != nil {
		return nil, err
	}
	appearances, err := readCSV(filepath.Join(dir, "ItemAppearance.csv"))
	if err != nil {
		return nil, err
	}
	appearanceIcon := map[int]int{}
	for _, r := range appearances {
		appearanceIcon[atoi(r["ID"])] = atoi(r["DefaultIconFileDataID"])
	}
	type pick struct{ modifier, order, appearance int }
	base := map[int]pick{}
	for _, r := range links {
		item, p := atoi(r["ItemID"]), pick{atoi(r["ItemAppearanceModifierID"]), atoi(r["OrderIndex"]), atoi(r["ItemAppearanceID"])}
		if cur, ok := base[item]; !ok || p.modifier < cur.modifier || (p.modifier == cur.modifier && p.order < cur.order) {
			base[item] = p
		}
	}
	icons := map[int]int{}
	for _, r := range items {
		id := atoi(r["ID"])
		if fdid := atoi(r["IconFileDataID"]); fdid > 0 {
			icons[id] = fdid
		} else if fdid := appearanceIcon[base[id].appearance]; fdid > 0 {
			icons[id] = fdid
		}
	}
	return icons, nil
}
