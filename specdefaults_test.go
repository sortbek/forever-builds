package main

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/wowsims/forever/sim/core/proto"
)

func TestEveryDefaultsFileParsesStrictly(t *testing.T) {
	for key := range specDirs {
		d, err := loadSpecDefaults(key)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		player := &proto.Player{}
		spec := player.ProtoReflect().Mutable(specField(key)).Message()
		options := spec.Mutable(spec.Descriptor().Fields().ByName("options")).Message()
		for name, pair := range map[string]struct {
			raw []byte
			msg protoMessage
		}{
			"specOptions":     {d.SpecOptions, options.Interface()},
			"consumables":     {d.Consumables, &proto.ConsumesSpec{}},
			"raidBuffs":       {d.RaidBuffs, &proto.RaidBuffs{}},
			"partyBuffs":      {d.PartyBuffs, &proto.PartyBuffs{}},
			"individualBuffs": {d.IndividualBuffs, &proto.IndividualBuffs{}},
			"debuffs":         {d.Debuffs, &proto.Debuffs{}},
		} {
			if err := protojson.Unmarshal(pair.raw, pair.msg); err != nil {
				t.Errorf("%s %s: %v", key, name, err)
			}
		}
		if d.Other.ReactionTimeMs <= 0 || d.Talents == "" {
			t.Errorf("%s: other %+v talents %q", key, d.Other, d.Talents)
		}
	}
}

func TestHunterDefaultsCarryPetAndDistance(t *testing.T) {
	d, err := loadSpecDefaults("hunter")
	if err != nil {
		t.Fatal(err)
	}
	opts := &proto.Hunter_Options{}
	if err := protojson.Unmarshal(d.SpecOptions, opts); err != nil {
		t.Fatal(err)
	}
	if opts.GetClassOptions().GetPetType() != proto.HunterOptions_Cat || d.Other.DistanceFromTarget != 12 {
		t.Errorf("pet %v distance %v", opts.GetClassOptions().GetPetType(), d.Other.DistanceFromTarget)
	}
}
