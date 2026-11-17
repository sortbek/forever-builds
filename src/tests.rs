//! The release gate: every build sims with its defaults, on talents read from the UI data's trees.

use std::{fs, path::Path};

use serde_json::{json, Value};

use crate::{bulk, defaults, engine};

fn repo(path: &str) -> Value {
    let full = Path::new(env!("CARGO_MANIFEST_DIR")).join(path);
    let text = fs::read_to_string(&full).unwrap_or_else(|e| panic!("{}: {e}", full.display()));
    serde_json::from_str(&text).unwrap()
}

fn class_of(spec_key: &str) -> &'static str {
    match spec_key {
        "balanceDruid" | "feralCatDruid" | "feralBearDruid" => "druid",
        "hunter" => "hunter",
        "mage" => "mage",
        "protectionPaladin" | "retributionPaladin" => "paladin",
        "dpsPriest" => "priest",
        "rogue" => "rogue",
        "elementalShaman" | "enhancementShaman" => "shaman",
        "warlock" => "warlock",
        _ => "warrior",
    }
}

/// 51 points down one tree, row by row, in the order of the calculator the app shows.
fn talents(class: &str, tree: usize) -> String {
    let trees = repo(&format!("uidata/ui/sim/talents/trees/{class}.json"));
    let list = trees[tree]["talents"].as_array().unwrap();
    let mut order: Vec<usize> = (0..list.len()).collect();
    order.sort_by_key(|&i| list[i]["location"]["rowIdx"].as_u64().unwrap());
    let mut ranks = vec![0u64; list.len()];
    let mut left = 51;
    for i in order {
        ranks[i] = list[i]["maxPoints"].as_u64().unwrap().min(left);
        left -= ranks[i];
    }
    let digits: String = ranks.iter().map(u64::to_string).collect();
    format!("{}{digits}", "-".repeat(tree))
}

/// What SimHammer sends: the character, gear, talents and fight settings.
fn request(build: &str, talents: &str, iterations: u64) -> Value {
    let spec_key = defaults::all_defaults()[build]["specKey"].as_str().unwrap();
    let class = class_of(spec_key);
    let gear = repo(&format!("testdata/gear/{build}.json"));
    let mut player = json!({
        "name": build,
        "race": gear["race"],
        "class": format!("Class{}{}", class[..1].to_uppercase(), &class[1..]),
        "equipment": gear["equipment"],
        "talentsString": talents,
    });
    player[spec_key] = json!({});
    json!({
        "raid": { "parties": [{ "players": [player] }] },
        "encounter": { "duration": 180.0 },
        "simOptions": { "iterations": iterations, "randomSeed": "7" },
    })
}

/// The tree each build's talents sit in.
const BUILDS: [(&str, usize); 24] = [
    ("balance-druid", 0),
    ("feral-druid", 1),
    ("feral-bear-druid", 1),
    ("hunter", 0),
    ("marksmanship-hunter", 1),
    ("survival-hunter", 2),
    ("arcane-mage", 0),
    ("fire-mage", 1),
    ("frost-mage", 2),
    ("protection-paladin", 1),
    ("retribution-paladin", 2),
    ("smite-priest", 1),
    ("shadow-priest", 2),
    ("assassination-rogue", 0),
    ("combat-rogue", 1),
    ("subtlety-rogue", 2),
    ("elemental-shaman", 0),
    ("enhancement-shaman", 1),
    ("affliction-warlock", 0),
    ("demonology-warlock", 1),
    ("destruction-warlock", 2),
    ("arms-warrior", 0),
    ("warrior", 1),
    ("protection-warrior", 2),
];

#[test]
fn every_build_has_a_row_here() {
    let defaults = defaults::all_defaults().as_object().unwrap();
    assert_eq!(defaults.len(), BUILDS.len());
    for (build, _) in BUILDS {
        assert!(defaults.contains_key(build), "{build}");
    }
}

#[test]
fn every_build_sims_with_its_defaults() {
    let mut failures = Vec::new();
    for (build, tree) in BUILDS {
        let spec_key = defaults::all_defaults()[build]["specKey"].as_str().unwrap();
        let talents = talents(class_of(spec_key), tree);
        if defaults::build_for(spec_key, &talents) != Some(build) {
            failures.push(format!(
                "{build}: talents {talents} resolve to another build"
            ));
            continue;
        }
        let mut req = request(build, &talents, 300);
        let outcome = defaults::apply(&mut req).and_then(|()| engine::simulate(&req));
        match outcome {
            Ok(result) => {
                let unit = &result["raidMetrics"]["parties"][0]["players"][0];
                let dps = unit["dps"]["avg"].as_f64().unwrap_or(0.0);
                let pets: f64 = unit["pets"]
                    .as_array()
                    .into_iter()
                    .flatten()
                    .filter_map(|p| p["dps"]["avg"].as_f64())
                    .sum();
                println!("{build:22} dps {dps:7.1} pets {pets:6.1}");
                if dps <= 0.0 || pets > dps || (build == "hunter" && pets <= 0.0) {
                    failures.push(format!("{build}: dps {dps:.1}, pets {pets:.1}"));
                }
            }
            Err(e) => failures.push(format!("{build}: {e}")),
        }
    }
    assert!(failures.is_empty(), "\n{}", failures.join("\n"));
}

/// The trees no build's talents sit in still have to read in the calculator's order.
#[test]
fn the_other_trees_are_read_too() {
    for (build, tree) in [
        ("balance-druid", 2),
        ("retribution-paladin", 0),
        ("shadow-priest", 0),
        ("elemental-shaman", 2),
    ] {
        let spec_key = defaults::all_defaults()[build]["specKey"].as_str().unwrap();
        let mut req = request(build, &talents(class_of(spec_key), tree), 50);
        defaults::apply(&mut req).unwrap();
        if let Err(e) = engine::simulate(&req) {
            assert!(!e.contains("talents"), "{build} tree {tree}: {e}");
        }
    }
}

#[test]
fn what_the_request_sets_is_kept() {
    let mut req = request("frost-mage", "", 300);
    req["raid"]["parties"][0]["players"][0]["rotation"] = json!({ "type": "TypeAPL" });
    defaults::apply(&mut req).unwrap();
    let player = &req["raid"]["parties"][0]["players"][0];
    assert_eq!(req["encounter"]["duration"], 180.0);
    assert_eq!(req["encounter"]["targets"].as_array().unwrap().len(), 1);
    assert_eq!(req["simOptions"]["iterations"], 300);
    assert_eq!(player["rotation"], json!({ "type": "TypeAPL" }));
    assert!(player["consumables"].is_object() && player["mage"]["options"].is_object());
}

/// A level 29 hunter as SimHammer sends it: the sheet from the export, one pet option, no buffs.
fn levelling_hunter() -> Value {
    let mut req = request("hunter", "5120221402100000", 300);
    let player = &mut req["raid"]["parties"][0]["players"][0];
    let items = json!([
        {"id": 3732}, {"id": 5254}, {"id": 5314}, {"id": 270043, "enchant": 15}, {"id": 4794},
        {"id": 5299, "enchant": 15}, {"id": 4246}, {"id": 4831}, {"id": 3741, "enchant": 15},
        {"id": 6414}, {"id": 17694}, {"id": 3210, "randomSuffix": 12726}, {"id": 273843},
    ]);
    player["race"] = json!("RaceOrc");
    player["equipment"] = json!({ "items": items });
    player["level"] = json!(29);
    player["characterSheet"] = json!({
        "level": 29, "base": [36, 58, 50, 33, 43], "attributes": [54, 93, 83, 33, 61],
        "attributeBuffs": [18, 35, 33, 0, 18], "health": 1028, "mana": 960,
        "meleeCrit": 3.81, "rangedCrit": 4.01, "spellCrit": [4.65, 4.65, 4.65, 4.65, 4.65, 4.65],
        "dodge": 7.45, "parry": 4.36, "attackPower": [185, 6, 0], "rangedAttackPower": [224, 41, 0],
        "armor": [44, 712], "manaRegen": [13.88, 0.0], "auras": [14318, 19883],
        "equipment": { "items": items },
    });
    player["hunter"] = json!({ "options": { "classOptions": { "petType": "Raptor" } } });
    req
}

#[test]
fn a_levelling_character_sims_solo_against_a_target_near_its_level() {
    let mut req = levelling_hunter();
    defaults::apply(&mut req).unwrap();
    let target = &req["encounter"]["targets"][0];
    assert_eq!(
        (&target["level"], &target["name"]),
        (&json!(31), &json!("Level 31"))
    );
    assert_eq!(target["stats"][30], 1836.0);
    let player = &req["raid"]["parties"][0]["players"][0];
    assert_eq!(req["raid"]["buffs"], json!({}));
    assert_eq!(req["raid"]["debuffs"], json!({}));
    assert_eq!(player["consumables"], json!({}));
    let options = &player["hunter"]["options"]["classOptions"];
    assert_eq!(options["petType"], "Raptor");
    assert!(
        options["ammo"].is_string(),
        "the build's other options stay"
    );
    let result = engine::simulate(&req).unwrap();
    let unit = &result["raidMetrics"]["parties"][0]["players"][0];
    assert!(unit["dps"]["avg"].as_f64().unwrap() > 0.0);
    assert!(unit["pets"][0]["dps"]["avg"].as_f64().unwrap() > 0.0);
}

#[test]
fn an_empty_buff_set_stays_empty_at_60() {
    let mut req = request("frost-mage", "", 300);
    req["raid"]["buffs"] = json!({});
    defaults::apply(&mut req).unwrap();
    assert_eq!(req["raid"]["buffs"], json!({}));
    assert!(!req["raid"]["debuffs"].as_object().unwrap().is_empty());
}

#[test]
fn options_list_each_spec_with_its_builds_starting_state() {
    let out = defaults::options(
        &json!({ "specKeys": ["mage", "holyPaladin"], "talentsString": talents("mage", 2), "level": 60 }),
    );
    assert!(out.get("holyPaladin").is_none());
    let mage = &out["mage"];
    assert_eq!(mage["build"], "frost-mage");
    assert_eq!(mage["consumables"], true);
    let toggles = mage["toggles"].as_array().unwrap();
    let find = |id: &str| {
        toggles
            .iter()
            .find(|t| t["id"] == id)
            .unwrap_or_else(|| panic!("{id}"))
    };
    assert_eq!(find("raid.arcaneBrilliance")["on"], true);
    assert_eq!(find("raid.arcaneBrilliance")["label"], "Arcane Brilliance");
    assert_eq!(find("raid.giftOfTheWild")["label"], "Gift of the Wild");
    assert_eq!(find("debuff.sunderArmor")["on"], false);
    assert_eq!(find("debuff.curseOfElements")["group"], "debuff");
    assert!(toggles
        .iter()
        .all(|t| !t["id"].as_str().unwrap().contains("Resistance")));

    let levelling =
        defaults::options(&json!({ "specKeys": ["mage"], "talentsString": "", "level": 29 }));
    assert_eq!(levelling["mage"]["consumables"], false);
    assert!(levelling["mage"]["toggles"]
        .as_array()
        .unwrap()
        .iter()
        .all(|t| t["on"] == false));
}

#[test]
fn chosen_buffs_replace_the_builds_sets() {
    let mut req = request("warrior", &talents("warrior", 1), 300);
    req["simhammer"] = json!({ "buffs": ["party.battleShout", "raid.thorns", "debuff.faerieFire"], "consumables": false });
    defaults::apply(&mut req).unwrap();
    assert!(req.get("simhammer").is_none());
    let party = &req["raid"]["parties"][0];
    assert_eq!(req["raid"]["buffs"], json!({ "thorns": true }));
    assert_eq!(req["raid"]["debuffs"], json!({ "faerieFire": true }));
    assert_eq!(
        party["buffs"],
        json!({ "battleShout": defaults::all_defaults()["warrior"]["partyBuffs"]["battleShout"] })
    );
    assert_eq!(party["players"][0]["buffs"], json!({}));
    assert_eq!(party["players"][0]["consumables"], json!({}));
    engine::simulate(&req).unwrap();
}

#[test]
fn every_builds_starting_toggles_sim_like_its_defaults() {
    for (build, tree) in BUILDS {
        let spec_key = defaults::all_defaults()[build]["specKey"].as_str().unwrap();
        let talents = talents(class_of(spec_key), tree);
        let options = defaults::options(
            &json!({ "specKeys": [spec_key], "talentsString": talents, "level": 60 }),
        );
        let on: Vec<&Value> = options[spec_key]["toggles"]
            .as_array()
            .unwrap()
            .iter()
            .filter(|t| t["on"] == true)
            .map(|t| &t["id"])
            .collect();
        let mut chosen = request(build, &talents, 200);
        chosen["simhammer"] = json!({ "buffs": on });
        defaults::apply(&mut chosen).unwrap();
        if let Err(e) = engine::simulate(&chosen) {
            panic!("{build}: {e}");
        }
    }
}

#[test]
fn a_request_without_a_seed_sims() {
    let mut req = request("frost-mage", &talents("mage", 2), 200);
    req["simOptions"]
        .as_object_mut()
        .unwrap()
        .remove("randomSeed");
    defaults::apply(&mut req).unwrap();
    engine::simulate(&req).unwrap();
}

#[test]
fn a_dagger_combat_rogue_backstabs() {
    let dagger = defaults::items()
        .as_object()
        .unwrap()
        .iter()
        .find(|(_, item)| item[2] == "WeaponTypeDagger" && item[1] == "HandTypeMainHand")
        .map(|(id, _)| id.parse::<u64>().unwrap())
        .unwrap();
    let mut swords = request("combat-rogue", &talents("rogue", 1), 300);
    let mut daggers = swords.clone();
    daggers["raid"]["parties"][0]["players"][0]["equipment"]["items"][14] = json!({ "id": dagger });
    defaults::apply(&mut swords).unwrap();
    defaults::apply(&mut daggers).unwrap();
    let rotation = |r: &Value| r["raid"]["parties"][0]["players"][0]["rotation"].clone();
    assert_ne!(rotation(&swords), rotation(&daggers));
}

#[test]
fn healers_are_refused_by_name() {
    let mut req = json!({ "raid": { "parties": [{ "players": [{ "holyPaladin": {} }] }] } });
    assert_eq!(
        defaults::apply(&mut req).unwrap_err(),
        "healer specs can't be simmed yet"
    );
}

fn bulk_request() -> Value {
    // A Combat rogue with the Assassination rogue's gear in the bags.
    let bags = repo("testdata/gear/assassination-rogue.json");
    let worn = repo("testdata/gear/combat-rogue.json");
    let worn_ids: Vec<&Value> = worn["equipment"]["items"]
        .as_array()
        .unwrap()
        .iter()
        .map(|i| &i["id"])
        .collect();
    let items: Vec<Value> = bags["equipment"]["items"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|i| i["id"].as_u64().unwrap_or(0) > 0 && !worn_ids.contains(&&i["id"]))
        .take(4)
        .cloned()
        .collect();
    assert!(items.len() >= 2, "the two rogues wear the same gear");
    json!({ "baseRequest": request("combat-rogue", &talents("rogue", 1), 200), "bulkSettings": { "items": items } })
}

#[test]
fn bulk_ranks_the_gear_sets_against_the_worn_gear() {
    let result = bulk::run(bulk_request(), 0).unwrap();
    assert_eq!(result["baseline"]["candidateIndex"], -1);
    assert_eq!(
        result["baseline"]["gear"]["items"]
            .as_array()
            .unwrap()
            .len(),
        17
    );
    assert!(result["baseline"]["dpsMetrics"]["avg"].as_f64().unwrap() > 0.0);
    assert_eq!(result["baseline"]["dpsMetrics"]["aggregatorData"]["n"], 200);
    let top = result["topResults"].as_array().unwrap();
    assert!(!top.is_empty());
    let dps: Vec<f64> = top
        .iter()
        .map(|r| r["dpsMetrics"]["avg"].as_f64().unwrap())
        .collect();
    assert!(dps.windows(2).all(|w| w[0] >= w[1]), "{dps:?}");
    for row in top {
        assert_ne!(row["gear"]["items"], result["baseline"]["gear"]["items"]);
    }
}

#[test]
fn bulk_refuses_more_gear_sets_than_the_limit() {
    let error = bulk::run(bulk_request(), 1).unwrap_err();
    assert_eq!(error, "More than 1 combinations. Select fewer items.");
}
