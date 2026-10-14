package main

import (
	"encoding/json"
	"errors"

	"google.golang.org/protobuf/encoding/protojson"
	gproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/wowsims/forever/sim/core/proto"
	"github.com/wowsims/forever/sim/core/stats"
)

const defaultIterations = 12500

// The engine UI's default target (sim/encounters/forever_raids.go "Level 60").
func level60Target() *proto.Target {
	return &proto.Target{
		Id: 213336, Name: "Level 60", Level: 63, MobType: proto.MobType_MobTypeUnknown,
		Stats:         stats.Stats{stats.Health: 127_393, stats.Armor: 3731, stats.AttackPower: 805}.ToProtoArray(),
		SpellSchool:   proto.SpellSchool_SpellSchoolPhysical,
		SwingSpeed:    2,
		MinBaseDamage: 3000,
		DamageSpread:  0.3333,
		ParryHaste:    true,
	}
}

func applyEncounterDefaults(e *proto.Encounter) {
	if e.Duration == 0 {
		e.Duration = 120
	}
	if e.DurationVariation == 0 {
		e.DurationVariation = 15
	}
	if e.ExecuteProportion_20+e.ExecuteProportion_25+e.ExecuteProportion_35+e.ExecuteProportion_45+e.ExecuteProportion_90 == 0 {
		e.ExecuteProportion_20, e.ExecuteProportion_25, e.ExecuteProportion_35 = 0.2, 0.25, 0.35
		e.ExecuteProportion_45, e.ExecuteProportion_90 = 0.45, 0.9
	}
	if len(e.Targets) == 0 {
		e.Targets = []*proto.Target{level60Target()}
	}
}

func fillFrom(raw json.RawMessage, m protoMessage) error {
	if len(raw) == 0 {
		return nil
	}
	return protojson.Unmarshal(raw, m.ProtoReflect().Interface())
}

// Spec constructors dereference Options and its nested messages, so none may be nil.
func fillMessages(m protoreflect.Message, depth int) {
	if depth > 4 {
		return
	}
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.Kind() != protoreflect.MessageKind || fd.IsList() || fd.IsMap() || fd.ContainingOneof() != nil {
			continue
		}
		fillMessages(m.Mutable(fd).Message(), depth+1)
	}
}

func applyDefaults(req *proto.RaidSimRequest) error {
	if len(req.GetRaid().GetParties()) == 0 || len(req.Raid.Parties[0].Players) == 0 {
		return errors.New("request has no player")
	}
	party := req.Raid.Parties[0]
	player := party.Players[0]
	m := player.ProtoReflect()
	fd := m.WhichOneof(m.Descriptor().Oneofs().ByName("spec"))
	if fd == nil {
		return errors.New("player has no spec")
	}
	key := fd.JSONName()
	if _, ok := specDirs[key]; !ok {
		return errUnsupportedSpec
	}
	d, err := loadSpecDefaults(key)
	if err != nil {
		return err
	}

	if req.SimOptions == nil {
		req.SimOptions = &proto.SimOptions{}
	}
	if req.SimOptions.Iterations == 0 {
		req.SimOptions.Iterations = defaultIterations
	}
	req.Type = proto.SimType_SimTypeIndividual
	if req.Encounter == nil {
		req.Encounter = &proto.Encounter{}
	}
	applyEncounterDefaults(req.Encounter)

	spec := m.Mutable(fd).Message()
	optionsFd := spec.Descriptor().Fields().ByName("options")
	if !spec.Has(optionsFd) || gproto.Size(spec.Get(optionsFd).Message().Interface()) == 0 {
		if err := fillFrom(d.SpecOptions, spec.Mutable(optionsFd).Message().Interface()); err != nil {
			return err
		}
	}
	fillMessages(spec, 0)

	if player.Consumables == nil {
		player.Consumables = &proto.ConsumesSpec{}
		if err := fillFrom(d.Consumables, player.Consumables); err != nil {
			return err
		}
	}
	if player.Buffs == nil {
		player.Buffs = &proto.IndividualBuffs{}
		if err := fillFrom(d.IndividualBuffs, player.Buffs); err != nil {
			return err
		}
	}
	if party.Buffs == nil {
		party.Buffs = &proto.PartyBuffs{}
		if err := fillFrom(d.PartyBuffs, party.Buffs); err != nil {
			return err
		}
	}
	if req.Raid.Buffs == nil {
		req.Raid.Buffs = &proto.RaidBuffs{}
		if err := fillFrom(d.RaidBuffs, req.Raid.Buffs); err != nil {
			return err
		}
	}
	if req.Raid.Debuffs == nil {
		req.Raid.Debuffs = &proto.Debuffs{}
		if err := fillFrom(d.Debuffs, req.Raid.Debuffs); err != nil {
			return err
		}
	}

	if player.ReactionTimeMs == 0 {
		player.ReactionTimeMs = d.Other.ReactionTimeMs
	}
	if player.DistanceFromTarget == 0 {
		player.DistanceFromTarget = d.Other.DistanceFromTarget
	}
	if player.ChannelClipDelayMs == 0 {
		player.ChannelClipDelayMs = d.Other.ChannelClipDelayMs
	}
	if tankSpecs[key] {
		player.InFrontOfTarget = true
		if len(req.Raid.Tanks) == 0 {
			req.Raid.Tanks = []*proto.UnitReference{{Type: proto.UnitReference_Player, Index: 0}}
		}
	}

	if player.Rotation == nil {
		var mainHand int32
		if items := player.GetEquipment().GetItems(); len(items) > int(proto.ItemSlot_ItemSlotMainHand) {
			mainHand = items[proto.ItemSlot_ItemSlotMainHand].GetId()
		}
		path, err := aplPath(key, player.TalentsString, mainHand)
		if err != nil {
			return err
		}
		if player.Rotation, err = loadRotation(path); err != nil {
			return err
		}
	}
	return nil
}
