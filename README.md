# forever-builds

`simhammer-forever` is the command-line runner SimHammer uses to sim WoW Forever characters. It wraps the
[MythicSim Forever engine](https://github.com/sage3648/mythicsim-forever-engine) (MIT, Rust) and fills in each
build's defaults, so SimHammer only sends the character, gear, talents and fight settings.

```
simhammer-forever sim  --in request.json --out result.json     # RaidSimRequest in, RaidSimResult out
simhammer-forever bulk --in request.json --out result.json     # gear sets ranked against the worn gear
simhammer-forever version
```

## What comes from where

| Part | Source |
| --- | --- |
| Simulation, item and spell data, rotation presets | `engine/`: sage3648/mythicsim-forever-engine at `ENGINE_REF` |
| Talent calculator trees, item and spell icons | `uidata/`: wowsims/forever at `UIDATA_REF` |
| Where each item comes from (`sources.json`) | `att/`: ATTWoWAddon/AllTheThings' Forever database at `ATT_REF` |
| Trait ids, icon file ids, item quality | wago.tools tables of the newest live build (`wowf`) |
| Consumables, buffs, debuffs, spec options and target per build | `defaults/`, ours to edit |

`defaults/<build>.json` started as MythicSim's reference requests. A build is picked by spec and the tree with the
most talent points (`src/defaults.rs`), and its `rotation` names one of the engine's presets.

## Build

Needs Rust (the toolchain in `rust-toolchain.toml`), Go and Node.

```sh
bash scripts/sync-engine.sh     # check out engine/, uidata/ and att/
bash scripts/fetch-wago.sh      # client tables into wago/
cargo test                      # the gate: every build sims with its defaults
cargo build --release
go run ./cmd/gendata            # dist/data, the bundle SimHammer reads next to the binary
```

CI does this daily against the newest engine, UI data and item source commits and publishes a release for every platform
when the gate passes. When it fails nothing is released, and SimHammer keeps the last good engine.
