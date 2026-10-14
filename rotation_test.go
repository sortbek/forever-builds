package main

import "testing"

func TestAplPathPortsAutoRotation(t *testing.T) {
	const dagger, sword, twoHander = 18816, 19019, 12784 // Perdition's Blade, Thunderfury, Arcanite Reaper
	cases := []struct {
		spec, talents string
		mainHand      int32
		want          string
	}{
		{"balanceDruid", "", 0, "druid/balance/launch"},
		{"feralCatDruid", "", 0, "druid/feralcat/default"},
		{"feralBearDruid", "", 0, "druid/feralbear/default"},
		{"hunter", "--55", 0, "hunter/dps/mm"},
		{"mage", "55", 0, "mage/dps/arcane"},
		{"mage", "-55", 0, "mage/dps/fire"},
		{"mage", "--55", 0, "mage/dps/frost"},
		{"protectionPaladin", "", 0, "paladin/protection/p5"},
		{"retributionPaladin", "", 0, "paladin/retribution/default"},
		{"dpsPriest", "--000000000000000001", 0, "priest/dps/shadow"},
		{"dpsPriest", "5", 0, "priest/dps/smite"},
		{"rogue", "00000000000001", dagger, "rogue/dps/forever_mutilate"},
		{"rogue", "--0000000000000001", dagger, "rogue/dps/forever_hemorrhage"},
		{"rogue", "-5", dagger, "rogue/dps/combat_backstab"},
		{"rogue", "00000000000001", sword, "rogue/dps/combat_sinister_strike"},
		{"elementalShaman", "", 0, "shaman/elemental/forever"},
		{"enhancementShaman", "", 0, "shaman/enhancement/forever"},
		{"warlock", "-0000000000000000001", 0, "warlock/dps/demonic_pact"},
		{"warlock", "-0000000001-0000001", 0, "warlock/dps/ds_ruin"},
		{"warlock", "-0000000001-000000100000001", 0, "warlock/dps/destruction_conflag"},
		{"warlock", "5", 0, "warlock/dps/affliction"},
		{"warlock", "-5", 0, "warlock/dps/demonic_pact"},
		{"warlock", "--5", 0, "warlock/dps/ds_ruin"},
		{"dpsWarrior", "000000000005", twoHander, "warrior/dps/dps_battle"},
		{"dpsWarrior", "000000000005", sword, "warrior/dps/dps_dance"},
		{"dpsWarrior", "", twoHander, "warrior/dps/dps_dance"},
		{"protectionWarrior", "", 0, "warrior/protection/protection"},
	}
	for _, c := range cases {
		got, err := aplPath(c.spec, c.talents, c.mainHand)
		if err != nil || got != c.want {
			t.Errorf("%s %q mh=%d: got %q, %v; want %q", c.spec, c.talents, c.mainHand, got, err, c.want)
		}
	}
}

func TestEveryRotationFileLoads(t *testing.T) {
	for _, path := range []string{"druid/balance/launch", "paladin/protection/p5", "warlock/dps/ds_ruin", "warrior/dps/dps_dance"} {
		r, err := loadRotation(path)
		if err != nil || len(r.PriorityList) == 0 {
			t.Errorf("%s: %v", path, err)
		}
	}
}

func TestTalentPointsByName(t *testing.T) {
	points, perTree, err := talentPoints("warlock", "-0000000001-0000001")
	if err != nil {
		t.Fatal(err)
	}
	if points["demonicSacrifice"] != 1 || points["ruin"] != 1 || perTree != [3]int{0, 1, 1} {
		t.Errorf("points %v per tree %v", points, perTree)
	}
}
