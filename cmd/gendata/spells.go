package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type named struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var spellIDPattern = regexp.MustCompile(`SpellID:\s*(\d+)`)

// Spell ids the engine's Go code creates actions for; pet abilities, for one, have no UI icon entry.
func engineSpellIDs(simDir string) (map[int]bool, error) {
	ids := map[int]bool{}
	err := filepath.WalkDir(simDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range spellIDPattern.FindAllSubmatch(data, -1) {
			if id, err := strconv.Atoi(string(m[1])); err == nil {
				ids[id] = true
			}
		}
		return nil
	})
	return ids, err
}

func loadSpellNames(path string) (map[int]string, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	names := make(map[int]string, len(rows))
	for _, r := range rows {
		names[atoi(r["ID"])] = r["Name_lang"]
	}
	return names, nil
}

// The UI's spell names first, then the client's name for every other spell the engine acts with.
func spellList(icons []named, names map[int]string, used map[int]bool) []named {
	out := append([]named(nil), icons...)
	seen := map[int]bool{}
	for _, s := range icons {
		seen[s.ID] = true
	}
	for id := range used {
		if name, ok := names[id]; ok && !seen[id] && name != "" {
			out = append(out, named{id, name})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}
