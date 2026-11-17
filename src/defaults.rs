//! Fills a request with its build's defaults: spec options, consumables, buffs, debuffs, the
//! target and the rotation. SimHammer sends only the character, gear, talents and fight settings.

use std::{
    sync::OnceLock,
    time::{SystemTime, UNIX_EPOCH},
};

use serde_json::{json, Map, Value};

pub const DEFAULT_ITERATIONS: u64 = 12_500;
/// stats.Armor in a target's stats array.
const ARMOR: usize = 30;

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

/// [`fill`] through nested objects, so a request can set one option and keep the others.
fn fill_deep(into: &mut Value, from: &Value) {
    let into = object(into);
    for (key, value) in from.as_object().into_iter().flatten() {
        match into.get_mut(key) {
            Some(sent) if sent.is_object() && value.is_object() => fill_deep(sent, value),
            Some(_) => {}
            None => {
                into.insert(key.clone(), value.clone());
            }
        }
    }
}

/// The build's target two levels above a levelling character, its armor scaled to its level.
fn levelling_target(encounter: &Value, level: u64) -> Value {
    let mut target = encounter["targets"][0].clone();
    let target_level = level + 2;
    let full = (
        target["level"].as_u64().unwrap_or(63),
        target["stats"][ARMOR].as_f64().unwrap_or(0.0),
    );
    target["level"] = json!(target_level);
    target["name"] = json!(format!("Level {target_level}"));
    target["stats"][ARMOR] = json!((full.1 * target_level as f64 / full.0 as f64).round());
    target
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

/// [{ id: "<group>.<key>", label, on: the value that switches it on }].
pub fn toggles() -> &'static Value {
    static CELL: OnceLock<Value> = OnceLock::new();
    embedded(
        &CELL,
        include_str!(concat!(env!("OUT_DIR"), "/toggles.json")),
    )
}

/// Where a toggle group's set sits in the build's defaults.
fn default_set<'a>(defaults: &'a Value, group: &str) -> &'a Value {
    match group {
        "raid" => &defaults["raidBuffs"],
        "party" => &defaults["partyBuffs"],
        "player" => &defaults["player"]["buffs"],
        _ => &defaults["debuffs"],
    }
}

fn is_on(value: &Value) -> bool {
    !(value.is_null()
        || *value == json!(false)
        || *value == json!(0)
        || *value == json!("TristateEffectMissing"))
}

fn levelling_at(level: u64) -> bool {
    (10..60).contains(&level)
}

/// What the settings block offers per spec: each buff and debuff with its starting state.
/// Reads `{ specKeys, talentsString, level }`.
pub fn options(input: &Value) -> Value {
    let talents = input["talentsString"].as_str().unwrap_or("");
    let levelling = levelling_at(input["level"].as_u64().unwrap_or(60));
    let mut out = Map::new();
    for spec_key in input["specKeys"]
        .as_array()
        .into_iter()
        .flatten()
        .filter_map(Value::as_str)
    {
        let Some(build) = build_for(spec_key, talents) else {
            continue;
        };
        let defaults = &all_defaults()[build];
        let list: Vec<Value> = toggles()
            .as_array()
            .into_iter()
            .flatten()
            .map(|toggle| {
                let (group, key) = toggle["id"]
                    .as_str()
                    .unwrap_or(".")
                    .split_once('.')
                    .unwrap_or(("", ""));
                json!({
                    "id": toggle["id"],
                    "group": if group == "debuff" { "debuff" } else { "buff" },
                    "label": toggle["label"],
                    "on": !levelling && is_on(&default_set(defaults, group)[key]),
                })
            })
            .collect();
        out.insert(
            spec_key.to_string(),
            json!({ "build": build, "toggles": list, "consumables": !levelling }),
        );
    }
    Value::Object(out)
}

/// Turns the request's own `simhammer` choices into the sets the engine reads, and removes them.
fn apply_choices(request: &mut Value, defaults: &Value) {
    let Some(choices) = request.as_object_mut().and_then(|r| r.remove("simhammer")) else {
        return;
    };
    if let Some(on) = choices["buffs"].as_array() {
        let mut sets: [Map<String, Value>; 4] = Default::default();
        for toggle in toggles().as_array().into_iter().flatten() {
            if !on.contains(&toggle["id"]) {
                continue;
            }
            let (group, key) = toggle["id"]
                .as_str()
                .unwrap_or(".")
                .split_once('.')
                .unwrap_or(("", ""));
            // The build's own strength of the buff (an improved Battle Shout) where it has one.
            let preset = &default_set(defaults, group)[key];
            let value = if is_on(preset) { preset } else { &toggle["on"] };
            let set = ["raid", "party", "player", "debuff"]
                .iter()
                .position(|g| *g == group)
                .unwrap_or(3);
            sets[set].insert(key.to_string(), value.clone());
        }
        let [raid, party, player, debuffs] = sets;
        request["raid"]["buffs"] = Value::Object(raid);
        request["raid"]["debuffs"] = Value::Object(debuffs);
        request["raid"]["parties"][0]["buffs"] = Value::Object(party);
        request["raid"]["parties"][0]["players"][0]["buffs"] = Value::Object(player);
    }
    match choices["consumables"].as_bool() {
        Some(false) => request["raid"]["parties"][0]["players"][0]["consumables"] = json!({}),
        Some(true) => {
            request["raid"]["parties"][0]["players"][0]["consumables"] =
                defaults["player"]["consumables"].clone()
        }
        None => {}
    }
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
    let level = player["level"].as_u64().unwrap_or(60);
    let levelling = levelling_at(level);
    apply_choices(request, defaults);

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
    if levelling && request["encounter"].get("targets").is_none() {
        request["encounter"]["targets"] = json!([levelling_target(&defaults["encounter"], level)]);
    }
    fill(&mut request["encounter"], &defaults["encounter"]);

    // What the request sets is kept, an empty set too. A levelling character starts solo.
    let default_of = |key: &str| {
        if levelling {
            json!({})
        } else {
            defaults[key].clone()
        }
    };
    let raid = &mut request["raid"];
    for (key, from) in [("buffs", "raidBuffs"), ("debuffs", "debuffs")] {
        if raid.get(key).is_none() {
            raid[key] = default_of(from);
        }
    }
    if raid.get("tanks").is_none() && defaults["tanks"].as_array().is_some_and(|t| !t.is_empty()) {
        raid["tanks"] = defaults["tanks"].clone();
    }
    let party = &mut raid["parties"][0];
    if party.get("buffs").is_none() {
        party["buffs"] = default_of("partyBuffs");
    }

    let player = &mut party["players"][0];
    for (key, value) in defaults["player"].as_object().into_iter().flatten() {
        if key == spec_key {
            fill_deep(&mut player[spec_key], value);
        } else if player.get(key).is_none() {
            let solo = levelling && (key == "buffs" || key == "consumables");
            player[key.as_str()] = if solo { json!({}) } else { value.clone() };
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
