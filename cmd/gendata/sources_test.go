package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeATT(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files[".config/constants/maps.lua"] = "local _MAP = { THE_BARRENS = 1413; WARSONG_GULCH = 1460; }\nfor k, v in pairs(_MAP) do MAP[k] = v end\n"
	for name, body := range files {
		path := filepath.Join(dir, ".contrib/.db/forever", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestNameCommentsMovesNamesIntoCalls(t *testing.T) {
	src := "\t\te(97, {\t-- Baron \"Silver\" Laine\n\t\tn(3864),\t-- Fel Steed\n\t\ti(6323),\t-- Baron's Scepter\n"
	want := "\t\te(97, { __name = \"Baron \\\"Silver\\\" Laine\",\n\t\tn(3864, { __name = \"Fel Steed\" }),\n\t\ti(6323),\t-- Baron's Scepter\n"
	if got := nameComments(src); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSourcesFromEachKindOfCall(t *testing.T) {
	dir := writeATT(t, map[string]string{
		"dungeons & raids/keep.lua": `
instanceroot(64, {	-- Shadowfang Keep
	timeline = { TIMELINE.ADDED_1_60_1 },
	lvl = 14,
	groups = {
		n(QUESTS, {
			q(1014, {	-- Arugal Must Die
				races = HORDE_ONLY,
				lvl = 18,
				groups = { i(1) },
			}),
			{	-- Split Quest
				allianceQuestData = q(7065, {	-- Split Quest (A)
				}),
				hordeQuestData = q(7064, {	-- Split Quest (H)
				}),
				lvl = { 25, 34 },
				groups = { i(2) },
			},
		}),
		n(ZONE_DROPS, {
			i(3),
		}),
		e(97, {	-- Baron Silverlaine
			creatureID = 3887,
			groups = {
				i(4),
				i(99),	-- not gear
				--[[ i(5), ]]
				i(6, { u = REMOVED_FROM_GAME }),
			},
		}),
		n(3864, {	-- Fel Steed
			i(7),
		}),
	},
});`,
		"zones/barrens.lua": `
root(ROOTS.Zones, maproot(MAP.KALIMDOR, MAP.THE_BARRENS, {
	groups = {
		n(VENDORS, {
			n(3493, {	-- Grazlix <Armorer>
				groups = {
					i(8),
					i(9, { minReputation = { 2798, FRIENDLY } }),
					i(20),
				},
			}),
		}),
		n(RARES, {
			n(5797, {	-- Aean Swiftriver
				i(10, { classes = { HUNTER, ROGUE } }),
			}),
		}),
	},
}));`,
		"crafted items/base.lua": `
root(ROOTS.Craftables, {
	prof(BLACKSMITHING, {
		prof(ARMORSMITH, {
			i(11),
		}),
	}),
});`,
		"pvp/gear.lua": `
root(ROOTS.PVP, {
	n(FACTION_HEADER_HORDE, {
		i(20),
	}),
	n(PREMIER_PVP_GEAR, {
		i(21),
	}),
});`,
		"pvp/battlegrounds/wsg.lua": `
battleground(MAP.WARSONG_GULCH, {
	groups = {
		n(VENDORS, {
			n(14754, {	-- Kelm Hargunth
				races = HORDE_ONLY,
				groups = {
					i(12),
					i(13, { minReputation = { FACTION_WARSONG_OUTRIDERS, HONORED } }),
				},
			}),
		}),
	},
});`,
		"world drops/world drops.lua": `
local BOSSES = { 643, 644 };
root(ROOTS.WorldDrops, {
	filter(CLOTH, {
		i(14, { crs = BOSSES }),
	}),
});`,
		"profession db/alchemy.lua": "local i = GetRecipeHelperForProfession(ALCHEMY);\ni(0, 2259);\n",
		// A zone inside a root that is skipped is skipped with it.
		"secrets/never.lua": "root(ROOTS.NeverImplemented, maproot(MAP.KALIMDOR, MAP.THE_BARRENS, {\n\tn(RARES, { n(1, {\t-- Ghost\n\t\ti(15),\n\t}) }),\n}));\n",
	})
	gear := map[int]bool{}
	for id := 1; id <= 21; id++ {
		gear[id] = true
	}
	got, err := buildSources(dir, gear, map[int]string{1413: "The Barrens", 1460: "Warsong Gulch"}, map[int]string{2798: "Darkspear Raiders"})
	if err != nil {
		t.Fatal(err)
	}
	want := []*sourceGroup{
		{Category: "crafted", Name: "Blacksmithing", Items: []sourceItem{{ID: 11}}},
		{Category: "dungeon", Name: "Shadowfang Keep", Level: 14, New: true, Items: []sourceItem{
			{ID: 3, From: "Trash"}, {ID: 4, From: "Baron Silverlaine"}, {ID: 7, From: "Fel Steed"},
		}},
		{Category: "quest", Name: "Shadowfang Keep", Items: []sourceItem{
			{ID: 1, From: "Arugal Must Die", Side: "horde", Level: 18}, {ID: 2, From: "Split Quest", Level: 25},
		}},
		{Category: "pvp", Name: "Kelm Hargunth", Zone: "Warsong Gulch", Items: []sourceItem{{ID: 12, Side: "horde"}}},
		{Category: "reputation", Name: "Warsong Outriders", Items: []sourceItem{{ID: 13, From: "Kelm Hargunth", Side: "horde", Standing: "honored"}}},
		{Category: "pvp", Name: "PvP rank rewards", Items: []sourceItem{{ID: 20, Side: "horde"}}},
		{Category: "pvp", Name: "Premier PvP gear", Items: []sourceItem{{ID: 21}}},
		{Category: "world", Name: "World drops", Items: []sourceItem{{ID: 14}}},
		// Item 20 is also sold here, but it is listed under PvP.
		{Category: "vendor", Name: "Grazlix <Armorer>", Zone: "The Barrens", Items: []sourceItem{{ID: 8}}},
		{Category: "reputation", Name: "Darkspear Raiders", Items: []sourceItem{{ID: 9, From: "Grazlix <Armorer>", Standing: "friendly"}}},
		{Category: "world", Name: "The Barrens", Items: []sourceItem{{ID: 10, From: "Aean Swiftriver", Classes: []string{"hunter", "rogue"}}}},
	}
	if !reflect.DeepEqual(got, want) {
		for _, g := range got {
			t.Logf("got %+v", *g)
		}
		t.Fail()
	}
}
