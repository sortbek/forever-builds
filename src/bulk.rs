//! Gear bulk sim: every way to wear the candidate items, ranked by DPS. Reads a `BulkSimRequest`
//! (`baseRequest`, `bulkSettings.items`) and writes the parts of a `BulkSimResult` SimHammer reads.

use std::{
    collections::HashSet,
    sync::{
        atomic::{AtomicUsize, Ordering},
        Mutex,
    },
    thread,
    time::{SystemTime, UNIX_EPOCH},
};

use serde_json::{json, Value};

use crate::{defaults, engine};

const SLOTS: usize = 17;
const MAIN_HAND: usize = 14;
const OFF_HAND: usize = 15;
const DEFAULT_TOP_RESULTS: usize = 25;
/// Above this many gear sets a low-iteration pass picks the sets worth the full run.
const STAGED_ABOVE: usize = 50;

/// The equipment slots an item type goes in.
fn slots_of(item: &Value) -> &'static [usize] {
    match item[0].as_str().unwrap_or("") {
        "ItemTypeHead" => &[0],
        "ItemTypeNeck" => &[1],
        "ItemTypeShoulder" => &[2],
        "ItemTypeBack" => &[3],
        "ItemTypeChest" => &[4],
        "ItemTypeWrist" => &[5],
        "ItemTypeHands" => &[6],
        "ItemTypeWaist" => &[7],
        "ItemTypeLegs" => &[8],
        "ItemTypeFeet" => &[9],
        "ItemTypeFinger" => &[10, 11],
        "ItemTypeTrinket" => &[12, 13],
        "ItemTypeRanged" => &[16],
        "ItemTypeWeapon" => match item[1].as_str().unwrap_or("") {
            "HandTypeOffHand" => &[OFF_HAND],
            "HandTypeOneHand" => &[MAIN_HAND, OFF_HAND],
            _ => &[MAIN_HAND],
        },
        _ => &[],
    }
}

fn id_of(spec: &Value) -> u64 {
    spec["id"].as_u64().unwrap_or(0)
}

fn info(spec: &Value) -> &'static Value {
    &defaults::items()[id_of(spec).to_string()]
}

fn is_two_hand(spec: &Value) -> bool {
    info(spec)[1] == "HandTypeTwoHand"
}

/// The same items worn, wherever they sit.
fn key(gear: &[Value]) -> Vec<(u64, u64)> {
    let mut worn: Vec<(u64, u64)> = gear
        .iter()
        .filter(|s| id_of(s) > 0)
        .map(|s| (id_of(s), s["enchant"].as_u64().unwrap_or(0)))
        .collect();
    worn.sort_unstable();
    worn
}

fn valid(gear: &[Value]) -> bool {
    if is_two_hand(&gear[MAIN_HAND]) && id_of(&gear[OFF_HAND]) > 0 {
        return false;
    }
    let mut seen = HashSet::new();
    gear.iter()
        .filter(|s| id_of(s) > 0)
        .all(|s| seen.insert(id_of(s)) || info(s)[3] != true)
}

struct Search<'a> {
    candidates: &'a [Value],
    limit: usize,
    seen: HashSet<Vec<(u64, u64)>>,
    sets: Vec<Vec<Value>>,
}

impl Search<'_> {
    /// Every gear set from `slot` on; false once there are more than the limit allows.
    fn walk(&mut self, slot: usize, gear: &mut Vec<Value>, used: &mut Vec<bool>) -> bool {
        if slot == SLOTS {
            if valid(gear) && self.seen.insert(key(gear)) {
                self.sets.push(gear.clone());
            }
            return self.limit == 0 || self.sets.len() <= self.limit;
        }
        if !self.walk(slot + 1, gear, used) {
            return false;
        }
        let kept = gear[slot].clone();
        for i in 0..self.candidates.len() {
            let candidate = &self.candidates[i];
            if used[i] || !slots_of(info(candidate)).contains(&slot) || *candidate == kept {
                continue;
            }
            let off_hand = gear[OFF_HAND].clone();
            gear[slot] = candidate.clone();
            if slot == MAIN_HAND && is_two_hand(candidate) {
                gear[OFF_HAND] = json!({});
            }
            used[i] = true;
            let more = self.walk(slot + 1, gear, used);
            used[i] = false;
            gear[slot] = kept.clone();
            gear[OFF_HAND] = off_hand;
            if !more {
                return false;
            }
        }
        true
    }
}

fn with_gear(base: &Value, gear: &[Value], iterations: u64) -> Value {
    let mut request = base.clone();
    request["raid"]["parties"][0]["players"][0]["equipment"]["items"] = json!(gear);
    request["simOptions"]["iterations"] = json!(iterations);
    request
}

/// The player's DPS metrics; the engine counts the pets' damage in them.
fn dps_metrics(result: &Value, iterations: u64) -> Value {
    let mut metrics = result["raidMetrics"]["parties"][0]["players"][0]["dps"].clone();
    metrics["aggregatorData"] = json!({ "n": iterations });
    metrics
}

/// Sims the gear sets side by side; a set the engine refuses gives `None`.
fn sim_all(base: &Value, sets: &[&Vec<Value>], iterations: u64) -> Vec<Option<Value>> {
    let next = AtomicUsize::new(0);
    let done = AtomicUsize::new(0);
    let results = Mutex::new(vec![None; sets.len()]);
    let refused = Mutex::new(HashSet::new());
    let workers = thread::available_parallelism()
        .map_or(1, |n| n.get())
        .min(sets.len().max(1));
    thread::scope(|scope| {
        for _ in 0..workers {
            scope.spawn(|| loop {
                let i = next.fetch_add(1, Ordering::Relaxed);
                if i >= sets.len() {
                    break;
                }
                match engine::simulate(&with_gear(base, sets[i], iterations)) {
                    Ok(result) => {
                        results.lock().unwrap()[i] = Some(dps_metrics(&result, iterations))
                    }
                    Err(reason) => {
                        if refused.lock().unwrap().insert(reason.clone()) {
                            eprintln!("skipped a gear set: {reason}");
                        }
                    }
                }
                eprintln!(
                    "progress {} {}",
                    done.fetch_add(1, Ordering::Relaxed) + 1,
                    sets.len()
                );
            });
        }
    });
    results.into_inner().unwrap()
}

fn avg(metrics: &Value) -> f64 {
    metrics["avg"].as_f64().unwrap_or(0.0)
}

pub fn run(mut request: Value, max_combinations: usize) -> Result<Value, String> {
    let mut base = request["baseRequest"].take();
    if !base.is_object() {
        return Err("bulk request has no baseRequest".into());
    }
    defaults::apply(&mut base)?;
    // One seed for every gear set, so the sets differ by their gear and not by their luck.
    if base["simOptions"].get("randomSeed").is_none() {
        let seed = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_or(1, |d| d.subsec_nanos());
        base["simOptions"]["randomSeed"] = json!(seed.to_string());
    }
    let iterations = base["simOptions"]["iterations"]
        .as_u64()
        .unwrap_or(defaults::DEFAULT_ITERATIONS);
    let top = match request["topResults"].as_u64().unwrap_or(0) as usize {
        0 => DEFAULT_TOP_RESULTS,
        n => n,
    };

    let mut gear: Vec<Value> = base["raid"]["parties"][0]["players"][0]["equipment"]["items"]
        .as_array()
        .cloned()
        .unwrap_or_default();
    gear.resize(SLOTS, json!({}));
    let candidates: Vec<Value> = request["bulkSettings"]["items"]
        .as_array()
        .cloned()
        .unwrap_or_default();

    let mut search = Search {
        candidates: &candidates,
        limit: max_combinations,
        seen: HashSet::new(),
        sets: Vec::new(),
    };
    // The first set walked is the gear as worn.
    if !search.walk(0, &mut gear.clone(), &mut vec![false; candidates.len()]) {
        return Err(format!(
            "More than {max_combinations} combinations. Select fewer items."
        ));
    }
    let sets = search.sets;
    let (baseline_set, others) = sets.split_first().ok_or("no gear to sim")?;

    let baseline = sim_all(&base, &[baseline_set], iterations)
        .pop()
        .flatten()
        .ok_or("the Forever engine can't sim the equipped gear; see the log")?;

    let mut survivors: Vec<usize> = (0..others.len()).collect();
    if others.len() > STAGED_ABOVE {
        let low = (iterations / 10).clamp(200, iterations);
        let refs: Vec<&Vec<Value>> = others.iter().collect();
        let scores = sim_all(&base, &refs, low);
        survivors.retain(|&i| scores[i].is_some());
        survivors.sort_by(|&a, &b| {
            avg(scores[b].as_ref().unwrap()).total_cmp(&avg(scores[a].as_ref().unwrap()))
        });
        survivors.truncate(top.max(DEFAULT_TOP_RESULTS));
    }
    let refs: Vec<&Vec<Value>> = survivors.iter().map(|&i| &others[i]).collect();
    let finals = sim_all(&base, &refs, iterations);
    let mut ranked: Vec<(usize, Value)> = survivors
        .iter()
        .zip(finals)
        .filter_map(|(&i, metrics)| Some((i, metrics?)))
        .collect();
    ranked.sort_by(|a, b| avg(&b.1).total_cmp(&avg(&a.1)));
    ranked.truncate(top);

    let top_results: Vec<Value> = ranked
        .into_iter()
        .map(|(i, metrics)| json!({ "candidateIndex": i, "gear": { "items": others[i] }, "dpsMetrics": metrics }))
        .collect();
    Ok(json!({
        "baseline": { "candidateIndex": -1, "gear": { "items": baseline_set }, "dpsMetrics": baseline },
        "topResults": top_results,
        "error": null,
    }))
}
