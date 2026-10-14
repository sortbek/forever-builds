package main

// A literal port of each ui/specs/<class>/<spec>/spec.ts autoRotation (feral bear: its default
// TypeAPL rotation). rotation-lock.json changes when upstream edits one; re-port, then commit it.

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/wowsims/forever/sim/core"
	"github.com/wowsims/forever/sim/core/proto"
)

func specClass(specKey string) string {
	return strings.SplitN(specDirs[specKey], "/", 2)[0]
}

type talentTree struct {
	Talents []struct {
		FieldName string `json:"fieldName"`
	} `json:"talents"`
}

func talentPoints(class, talents string) (map[string]int, [3]int, error) {
	var perTree [3]int
	data, err := assets.ReadFile("assets/trees/" + class + ".json")
	if err != nil {
		return nil, perTree, fmt.Errorf("no talent tree for %s", class)
	}
	var trees []talentTree
	if err := json.Unmarshal(data, &trees); err != nil {
		return nil, perTree, err
	}
	points := map[string]int{}
	for ti, part := range strings.SplitN(talents, "-", 3) {
		for i, c := range part {
			if ti < len(trees) && i < len(trees[ti].Talents) && c >= '0' && c <= '9' {
				points[trees[ti].Talents[i].FieldName] = int(c - '0')
				perTree[ti] += int(c - '0')
			}
		}
	}
	return points, perTree, nil
}

// The first tree with the most points, as the UI's points.indexOf(Math.max(...points)).
func primaryTree(perTree [3]int) int {
	best := 0
	for i := 1; i < 3; i++ {
		if perTree[i] > perTree[best] {
			best = i
		}
	}
	return best
}

func aplPath(specKey, talents string, mainHandID int32) (string, error) {
	class := specClass(specKey)
	t, perTree, err := talentPoints(class, talents)
	if err != nil {
		return "", err
	}
	var mainHand *core.Item
	if mainHandID != 0 {
		mainHand = core.GetItemByID(mainHandID)
	}
	switch specKey {
	case "balanceDruid":
		return "druid/balance/launch", nil
	case "feralCatDruid":
		return "druid/feralcat/default", nil
	case "feralBearDruid":
		return "druid/feralbear/default", nil
	case "hunter":
		return "hunter/dps/mm", nil
	case "mage":
		return []string{"mage/dps/arcane", "mage/dps/fire", "mage/dps/frost"}[primaryTree(perTree)], nil
	case "protectionPaladin":
		return "paladin/protection/p5", nil
	case "retributionPaladin":
		return "paladin/retribution/default", nil
	case "dpsPriest":
		if t["shadowform"] > 0 {
			return "priest/dps/shadow", nil
		}
		return "priest/dps/smite", nil
	case "rogue":
		if mainHand != nil && mainHand.WeaponType == proto.WeaponType_WeaponTypeDagger {
			if t["mutilate"] > 0 {
				return "rogue/dps/forever_mutilate", nil
			}
			if t["hemorrhage"] > 0 {
				return "rogue/dps/forever_hemorrhage", nil
			}
			return "rogue/dps/combat_backstab", nil
		}
		return "rogue/dps/combat_sinister_strike", nil
	case "elementalShaman":
		return "shaman/elemental/forever", nil
	case "enhancementShaman":
		return "shaman/enhancement/forever", nil
	case "warlock":
		if t["demonicPact"] > 0 {
			return "warlock/dps/demonic_pact", nil
		}
		if t["demonicSacrifice"] > 0 && t["ruin"] > 0 {
			if t["shadowAndFlame"] > 0 {
				return "warlock/dps/destruction_conflag", nil
			}
			return "warlock/dps/ds_ruin", nil
		}
		return []string{"warlock/dps/affliction", "warlock/dps/demonic_pact", "warlock/dps/ds_ruin"}[primaryTree(perTree)], nil
	case "dpsWarrior":
		if t["bloodthrill"] == 5 && mainHand != nil && mainHand.HandType == proto.HandType_HandTypeTwoHand {
			return "warrior/dps/dps_battle", nil
		}
		return "warrior/dps/dps_dance", nil
	case "protectionWarrior":
		return "warrior/protection/protection", nil
	}
	return "", errUnsupportedSpec
}

func loadRotation(path string) (*proto.APLRotation, error) {
	data, err := assets.ReadFile("assets/apl/" + path + ".apl.json")
	if err != nil {
		return nil, fmt.Errorf("no rotation %s", path)
	}
	r := &proto.APLRotation{}
	if err := protojson.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("rotation %s: %w", path, err)
	}
	return r, nil
}
