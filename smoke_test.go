package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/wowsims/forever/sim/core"
	"github.com/wowsims/forever/sim/core/proto"
)

// The spec's own launch gear from the engine checkout (the gear the UI ranks with).
func launchGear(t *testing.T, specKey string) *proto.EquipmentSpec {
	files, _ := filepath.Glob(filepath.Join("engine/ui/specs", specDirs[specKey], "gear_sets/*.gear.json"))
	if len(files) == 0 {
		t.Skip("engine checkout missing; run scripts/sync-engine.sh")
	}
	sort.Strings(files)
	pick := files[0]
	for _, f := range files {
		if strings.Contains(strings.ToLower(filepath.Base(f)), "launch") {
			pick = f
			break
		}
	}
	data, err := os.ReadFile(pick)
	if err != nil {
		t.Fatal(err)
	}
	g := &proto.EquipmentSpec{}
	if err := protojson.Unmarshal(data, g); err != nil {
		t.Fatal(err)
	}
	return g
}

func specRequest(t *testing.T, specKey string) *proto.RaidSimRequest {
	d, err := loadSpecDefaults(specKey)
	if err != nil {
		t.Fatal(err)
	}
	class := proto.Class(proto.Class_value["Class"+strings.ToUpper(specClass(specKey)[:1])+specClass(specKey)[1:]])
	p := &proto.Player{
		Class: class, Race: core.ClassRaceCapabilities[class][0],
		Equipment: launchGear(t, specKey), TalentsString: d.Talents,
	}
	p.ProtoReflect().Mutable(specField(specKey))
	return &proto.RaidSimRequest{
		Raid:       &proto.Raid{Parties: []*proto.Party{{Players: []*proto.Player{p}}}},
		SimOptions: &proto.SimOptions{Iterations: 300, RandomSeed: 1},
	}
}

func TestEverySpecSimsWithItsDefaults(t *testing.T) {
	for specKey := range specDirs {
		res, err := runSimRequest(specRequest(t, specKey))
		if err != nil {
			t.Errorf("%s: %v", specKey, err)
			continue
		}
		u := res.RaidMetrics.Parties[0].Players[0]
		pet := 0.0
		for _, p := range u.Pets {
			pet += p.Dps.Avg
		}
		if u.Dps.Avg <= 0 || (specKey == "hunter" && pet <= 0) {
			t.Errorf("%s: dps %.1f pet %.1f", specKey, u.Dps.Avg, pet)
		}
	}
}

func TestSimCommandWritesAResult(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.json"), filepath.Join(dir, "out.json")
	if err := writeMessage(in, specRequest(t, "retributionPaladin")); err != nil {
		t.Fatal(err)
	}
	if err := runSim(in, out); err != nil {
		t.Fatal(err)
	}
	res := &proto.RaidSimResult{}
	if err := readMessage(out, res); err != nil {
		t.Fatal(err)
	}
	if res.RaidMetrics.Parties[0].Players[0].Dps.Avg <= 0 {
		t.Error("no dps")
	}
}
