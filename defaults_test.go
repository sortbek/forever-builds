package main

import (
	"testing"

	"github.com/wowsims/forever/sim/core/proto"
)

func playerRequest(spec any) *proto.RaidSimRequest {
	p := &proto.Player{Race: proto.Race_RaceHuman}
	switch s := spec.(type) {
	case *proto.Player_RetributionPaladin:
		p.Class, p.Spec, p.TalentsString = proto.Class_ClassPaladin, s, "550030022001--05225231001330321"
	case *proto.Player_ProtectionPaladin:
		p.Class, p.Spec = proto.Class_ClassPaladin, s
	case *proto.Player_Hunter:
		p.Class, p.Spec, p.Race = proto.Class_ClassHunter, s, proto.Race_RaceTroll
	case *proto.Player_HolyPaladin:
		p.Class, p.Spec = proto.Class_ClassPaladin, s
	}
	return &proto.RaidSimRequest{Raid: &proto.Raid{Parties: []*proto.Party{{Players: []*proto.Player{p}}}}}
}

func ret() *proto.Player_RetributionPaladin {
	return &proto.Player_RetributionPaladin{RetributionPaladin: &proto.RetributionPaladin{}}
}

func TestApplyDefaultsFillsWhatTheUIWould(t *testing.T) {
	req := playerRequest(ret())
	if err := applyDefaults(req); err != nil {
		t.Fatal(err)
	}
	p := req.Raid.Parties[0].Players[0]
	e := req.Encounter
	if req.SimOptions.GetIterations() != 12500 || req.Type != proto.SimType_SimTypeIndividual {
		t.Errorf("options %v type %v", req.SimOptions, req.Type)
	}
	if e.Duration != 120 || e.DurationVariation != 15 || e.ExecuteProportion_20 != 0.2 || e.ExecuteProportion_90 != 0.9 {
		t.Errorf("encounter %v", e)
	}
	if len(e.Targets) != 1 || e.Targets[0].Level != 63 {
		t.Errorf("targets %v", e.Targets)
	}
	if p.GetRetributionPaladin().GetOptions() == nil {
		t.Error("spec options left nil")
	}
	if p.Consumables.GetFlaskId() != 13512 || !req.Raid.Buffs.GetArcaneBrilliance() || !req.Raid.Debuffs.GetJudgementOfTheCrusader() {
		t.Error("presets not applied")
	}
	if p.ReactionTimeMs != 200 || p.DistanceFromTarget != 5 {
		t.Errorf("reaction %d distance %v", p.ReactionTimeMs, p.DistanceFromTarget)
	}
	if p.Rotation == nil || len(p.Rotation.PriorityList) == 0 {
		t.Error("rotation not filled")
	}
	if p.InFrontOfTarget || len(req.Raid.Tanks) != 0 {
		t.Error("dps spec set up as a tank")
	}
}

func TestApplyDefaultsKeepsWhatTheRequestSets(t *testing.T) {
	req := playerRequest(ret())
	req.SimOptions = &proto.SimOptions{Iterations: 500}
	req.Encounter = &proto.Encounter{Duration: 300}
	req.Raid.Parties[0].Players[0].Consumables = &proto.ConsumesSpec{FoodId: 1}
	if err := applyDefaults(req); err != nil {
		t.Fatal(err)
	}
	if req.SimOptions.Iterations != 500 || req.Encounter.Duration != 300 {
		t.Error("request values overwritten")
	}
	if req.Raid.Parties[0].Players[0].Consumables.GetFlaskId() != 0 {
		t.Error("consumables preset applied over explicit consumables")
	}
	if !req.Raid.Buffs.GetArcaneBrilliance() {
		t.Error("buffs skipped because consumables were set")
	}
}

func TestApplyDefaultsSetsUpTanks(t *testing.T) {
	req := playerRequest(&proto.Player_ProtectionPaladin{ProtectionPaladin: &proto.ProtectionPaladin{}})
	if err := applyDefaults(req); err != nil {
		t.Fatal(err)
	}
	p := req.Raid.Parties[0].Players[0]
	if !p.InFrontOfTarget || len(req.Raid.Tanks) != 1 || req.Raid.Tanks[0].Type != proto.UnitReference_Player {
		t.Errorf("in front %v tanks %v", p.InFrontOfTarget, req.Raid.Tanks)
	}
}

func TestApplyDefaultsGivesHuntersPetAndRange(t *testing.T) {
	req := playerRequest(&proto.Player_Hunter{Hunter: &proto.Hunter{}})
	if err := applyDefaults(req); err != nil {
		t.Fatal(err)
	}
	p := req.Raid.Parties[0].Players[0]
	if p.GetHunter().GetOptions().GetClassOptions().GetPetType() != proto.HunterOptions_Cat || p.DistanceFromTarget != 12 {
		t.Errorf("pet %v distance %v", p.GetHunter().GetOptions(), p.DistanceFromTarget)
	}
}

func TestApplyDefaultsRejectsHealers(t *testing.T) {
	req := playerRequest(&proto.Player_HolyPaladin{HolyPaladin: &proto.HolyPaladin{}})
	if err := applyDefaults(req); err != errUnsupportedSpec {
		t.Errorf("err = %v", err)
	}
}
