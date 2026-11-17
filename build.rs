//! Embeds what the CLI needs from the engine checkout and this repo: item slots, the rotation
//! presets, our per-build defaults and the version strings.

use std::{collections::BTreeMap, env, fs, path::Path, process::Command};

use serde_json::{json, Value};

fn read_json(path: &Path) -> Value {
    let text = fs::read_to_string(path).unwrap_or_else(|e| panic!("{}: {e}", path.display()));
    serde_json::from_str(&text).unwrap_or_else(|e| panic!("{}: {e}", path.display()))
}

fn main() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let out = env::var("OUT_DIR").unwrap();
    for watched in [
        "engine/data/items.jsonl",
        "engine/presets",
        "defaults",
        "ENGINE_REF",
        "wago/BUILD",
    ] {
        println!("cargo:rerun-if-changed={watched}");
    }
    println!("cargo:rerun-if-env-changed=SIMHAMMER_FOREVER_VERSION");

    // id -> [type, hand type, weapon type, unique]; the bulk sim slots candidates by these.
    let mut items = BTreeMap::new();
    let lines = fs::read_to_string(root.join("engine/data/items.jsonl"))
        .expect("engine/data/items.jsonl: run scripts/sync-engine.sh");
    for line in lines.lines().filter(|l| !l.trim().is_empty()) {
        let item: Value = serde_json::from_str(line).expect("items.jsonl line");
        let Some(kind) = item["type"].as_str() else {
            continue;
        };
        items.insert(
            item["id"].as_u64().expect("item id").to_string(),
            json!([
                kind,
                item["hand_type"],
                item["weapon_type"],
                item["unique"].as_bool().unwrap_or(false)
            ]),
        );
    }
    fs::write(
        format!("{out}/items.json"),
        serde_json::to_string(&items).unwrap(),
    )
    .unwrap();

    let mut defaults = BTreeMap::new();
    let mut rotations = BTreeMap::new();
    for entry in fs::read_dir(root.join("defaults")).expect("defaults dir") {
        let path = entry.unwrap().path();
        let build = path.file_stem().unwrap().to_str().unwrap().to_string();
        defaults.insert(build, read_json(&path));
    }
    // Every preset the engine ships, so a build can name any of them.
    let catalog = read_json(&root.join("engine/presets/catalog.json"));
    for spec in catalog["specs"]
        .as_object()
        .expect("catalog specs")
        .values()
    {
        for choice in spec["choices"].as_array().expect("catalog choices") {
            let id = choice["id"].as_str().expect("choice id");
            let file = root
                .join("engine/presets")
                .join(choice["file"].as_str().expect("choice file"));
            rotations.insert(id.to_string(), read_json(&file));
        }
    }
    for (build, d) in &defaults {
        let id = d["rotation"].as_str().unwrap_or_default();
        if !rotations.contains_key(id) {
            let file = root.join("engine/presets").join(format!("{id}.apl.json"));
            assert!(
                file.is_file(),
                "defaults/{build}.json names rotation {id}, which the engine does not ship"
            );
            rotations.insert(id.to_string(), read_json(&file));
        }
    }
    fs::write(
        format!("{out}/defaults.json"),
        serde_json::to_string(&defaults).unwrap(),
    )
    .unwrap();
    fs::write(
        format!("{out}/rotations.json"),
        serde_json::to_string(&rotations).unwrap(),
    )
    .unwrap();

    let commit = Command::new("git")
        .args(["-C", "engine", "rev-parse", "--short", "HEAD"])
        .current_dir(root)
        .output()
        .ok()
        .filter(|o| o.status.success())
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .unwrap_or_else(|| "unknown".into());
    let build = fs::read_to_string(root.join("wago/BUILD"))
        .map(|s| s.trim().to_string())
        .unwrap_or_else(|_| "unknown".into());
    println!("cargo:rustc-env=ENGINE_COMMIT={commit}");
    println!("cargo:rustc-env=CLIENT_BUILD={build}");
    println!(
        "cargo:rustc-env=CLI_VERSION={}",
        env::var("SIMHAMMER_FOREVER_VERSION").unwrap_or_else(|_| "dev".into())
    );
}
