//! Fills a request with its build's defaults: spec options, consumables, buffs, debuffs, the
//! target and the rotation. SimHammer sends only the character, gear, talents and fight settings.

use std::{
    sync::OnceLock,
    time::{SystemTime, UNIX_EPOCH},
};

use serde_json::{json, Map, Value};

pub const DEFAULT_ITERATIONS: u64 = 12_500;

fn embedded(cell: &'static OnceLock<Value>, text: &'static str) -> &'static Value {
    cell.get_or_init(|| serde_json::from_str(text).expect("embedded json"))
}

pub fn all_defaults() -> &'static Value {
    static CELL: OnceLock<Value> = OnceLock::new();
    embedded(
        &CELL,
        include_str!(concat!(env!("OUT_DIR"), "/defaults.json")),
    )
}

fn rotations() -> &'static Value {
    static CELL: OnceLock<Value> = OnceLock::new();
    embedded(
        &CELL,
        include_str!(concat!(env!("OUT_DIR"), "/rotations.json")),
    )
}

/// id -> [type, hand type, weapon type, unique].
pub fn items() -> &'static Value {
    static CELL: OnceLock<Value> = OnceLock::new();
    embedded(&CELL, include_str!(concat!(env!("OUT_DIR"), "/items.json")))
}

/// The Player.spec oneof names SimHammer may send.
const SPEC_KEYS: [&str; 14] = [
    "balanceDruid",
    "feralCatDruid",
    "feralBearDruid",
    "hunter",
    "mage",
    "protectionPaladin",
    "retributionPaladin",
    "dpsPriest",
    "rogue",
    "elementalShaman",
    "enhancementShaman",
    "warlock",
    "dpsWarrior",
    "protectionWarrior",
];

/// The first tree with the most points.
fn primary_tree(talents: &str) -> usize {
    let mut points = [0u32; 3];
    for (tree, part) in talents.splitn(3, '-').enumerate() {
        points[tree] = part.chars().filter_map(|c| c.to_digit(10)).sum();
    }
    (1..3).fold(0, |best, i| if points[i] > points[best] { i } else { best })
}

/// The build whose defaults and rotation a character gets, by its spec and talents.
pub fn build_for(spec_key: &str, talents: &str) -> Option<&'static str> {
    let tree = primary_tree(talents);
    Some(match spec_key {
        "balanceDruid" => "balance-druid",
        "feralCatDruid" => "feral-druid",
        "feralBearDruid" => "feral-bear-druid",
        "hunter" => ["hunter", "marksmanship-hunter", "survival-hunter"][tree],
        "mage" => ["arcane-mage", "fire-mage", "frost-mage"][tree],
        "protectionPaladin" => "protection-paladin",
        "retributionPaladin" => "retribution-paladin",
        "dpsPriest" => ["smite-priest", "smite-priest", "shadow-priest"][tree],
        "rogue" => ["assassination-rogue", "combat-rogue", "subtlety-rogue"][tree],
        "elementalShaman" => "elemental-shaman",
        "enhancementShaman" => "enhancement-shaman",
        "warlock" => [
            "affliction-warlock",
            "demonology-warlock",
            "destruction-warlock",
        ][tree],
        "dpsWarrior" => ["arms-warrior", "warrior", "warrior"][tree],
        "protectionWarrior" => "protection-warrior",
        _ => return None,
    })
}

fn object(value: &mut Value) -> &mut Map<String, Value> {
    if !value.is_object() {
        *value = json!({});
    }
    value.as_object_mut().expect("object")
}

/// Sets the keys of `from` that `into` lacks.
fn fill(into: &mut Value, from: &Value) {
    let into = object(into);
    for (key, value) in from.as_object().into_iter().flatten() {
        into.entry(key.clone()).or_insert_with(|| value.clone());
    }
}

fn is_empty(value: Option<&Value>) -> bool {
    value.and_then(Value::as_object).is_none_or(Map::is_empty)
}

fn main_hand_is_dagger(player: &Value) -> bool {
    let id = player["equipment"]["items"][14]["id"].as_u64().unwrap_or(0);
    items()[id.to_string()][2] == "WeaponTypeDagger"
}

/// A Combat rogue with a dagger in the main hand backstabs.
fn rotation_id(build: &str, defaults: &Value, player: &Value) -> String {
    if build == "combat-rogue" && main_hand_is_dagger(player) {
        return "rogue/dps/combat_backstab".into();
    }
    defaults["rotation"]
        .as_str()
        .unwrap_or_default()
        .to_string()
}

pub fn apply(request: &mut Value) -> Result<(), String> {
    let player = &request["raid"]["parties"][0]["players"][0];
    if !player.is_object() {
        return Err("request has no player".into());
    }
    let spec_key = *SPEC_KEYS
        .iter()
        .find(|key| player.get(**key).is_some())
        .ok_or("healer specs can't be simmed yet")?;
    let talents = player["talentsString"].as_str().unwrap_or("");
    let build = build_for(spec_key, talents).ok_or("healer specs can't be simmed yet")?;
    let defaults = &all_defaults()[build];
    let rotation = rotation_id(build, defaults, player);

    let sim_options = object(&mut request["simOptions"]);
    if sim_options
        .get("iterations")
        .and_then(Value::as_u64)
        .unwrap_or(0)
        == 0
    {
        sim_options.insert("iterations".into(), json!(DEFAULT_ITERATIONS));
    }
    // The engine wants a seed; a bulk sim then runs every gear set on this one.
    if !sim_options.contains_key("randomSeed") {
        let seed = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_or(1, |d| d.subsec_nanos().max(1));
        sim_options.insert("randomSeed".into(), json!(seed.to_string()));
    }
    fill(&mut request["encounter"], &defaults["encounter"]);

    let raid = &mut request["raid"];
    for (key, from) in [("buffs", "raidBuffs"), ("debuffs", "debuffs")] {
        if is_empty(raid.get(key)) {
            raid[key] = defaults[from].clone();
        }
    }
    if raid.get("tanks").is_none() && defaults["tanks"].as_array().is_some_and(|t| !t.is_empty()) {
        raid["tanks"] = defaults["tanks"].clone();
    }
    let party = &mut raid["parties"][0];
    if party.get("buffs").is_none() {
        party["buffs"] = defaults["partyBuffs"].clone();
    }

    let player = &mut party["players"][0];
    let sent_options = !is_empty(player[spec_key].get("options"));
    for (key, value) in defaults["player"].as_object().into_iter().flatten() {
        if key == spec_key {
            if !sent_options {
                player[spec_key] = value.clone();
            }
        } else if player.get(key).is_none() {
            player[key.as_str()] = value.clone();
        }
    }
    if player.get("rotation").is_none() {
        let preset = rotations()
            .get(&rotation)
            .ok_or_else(|| format!("no rotation {rotation}"))?;
        player["rotation"] = preset.clone();
    }
    Ok(())
}
