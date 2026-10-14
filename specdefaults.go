package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/wowsims/forever/sim/core/proto"
)

// Simmable spec keys (the Player.spec oneof's JSON names) and their ui/specs folder.
var specDirs = map[string]string{
	"balanceDruid":       "druid/balance",
	"feralCatDruid":      "druid/feralcat",
	"feralBearDruid":     "druid/feralbear",
	"hunter":             "hunter/dps",
	"mage":               "mage/dps",
	"protectionPaladin":  "paladin/protection",
	"retributionPaladin": "paladin/retribution",
	"dpsPriest":          "priest/dps",
	"rogue":              "rogue/dps",
	"elementalShaman":    "shaman/elemental",
	"enhancementShaman":  "shaman/enhancement",
	"warlock":            "warlock/dps",
	"dpsWarrior":         "warrior/dps",
	"protectionWarrior":  "warrior/protection",
}

var errUnsupportedSpec = errors.New("healer specs can't be simmed yet")

var tankSpecs = map[string]bool{"feralBearDruid": true, "protectionPaladin": true, "protectionWarrior": true}

type protoMessage = interface{ ProtoReflect() protoreflect.Message }

type specDefaults struct {
	SpecOptions     json.RawMessage `json:"specOptions"`
	Consumables     json.RawMessage `json:"consumables"`
	RaidBuffs       json.RawMessage `json:"raidBuffs"`
	PartyBuffs      json.RawMessage `json:"partyBuffs"`
	IndividualBuffs json.RawMessage `json:"individualBuffs"`
	Debuffs         json.RawMessage `json:"debuffs"`
	Other           struct {
		ReactionTimeMs     int32   `json:"reactionTimeMs"`
		DistanceFromTarget float64 `json:"distanceFromTarget"`
		ChannelClipDelayMs int32   `json:"channelClipDelayMs"`
	} `json:"other"`
	Talents string `json:"talents"`
}

func loadSpecDefaults(key string) (*specDefaults, error) {
	data, err := assets.ReadFile("assets/defaults/" + key + ".json")
	if err != nil {
		return nil, fmt.Errorf("no defaults for %s", key)
	}
	d := &specDefaults{}
	if err := json.Unmarshal(data, d); err != nil {
		return nil, fmt.Errorf("defaults for %s: %w", key, err)
	}
	return d, nil
}

func specField(key string) protoreflect.FieldDescriptor {
	return (&proto.Player{}).ProtoReflect().Descriptor().Oneofs().ByName("spec").Fields().ByJSONName(key)
}
