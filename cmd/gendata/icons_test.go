package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIconFileIDsFromItemThenAppearance(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Item.csv":                   "ID,ClassID,IconFileDataID\n1,4,111\n2,4,0\n3,4,0\n",
		"ItemModifiedAppearance.csv": "ID,ItemID,ItemAppearanceModifierID,ItemAppearanceID,OrderIndex\n10,2,1,900,0\n11,2,0,901,0\n12,3,0,0,0\n",
		"ItemAppearance.csv":         "ID,DisplayType,ItemDisplayInfoID,DefaultIconFileDataID\n900,0,0,999\n901,0,0,222\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := loadIconFileIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Item 1: its own icon. Item 2: the base appearance (modifier 0), not the variant. Item 3: none.
	if want := map[int]int{1: 111, 2: 222}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}
