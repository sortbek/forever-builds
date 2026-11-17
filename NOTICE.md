# Where this comes from

`simhammer-forever` is SimHammer's own runner around other people's work. All of it is under
the MIT license; the copyright lines are in [LICENSE](LICENSE), which ships in every release.

## MythicSim Forever engine

The simulation is the [MythicSim Forever engine](https://github.com/sage3648/mythicsim-forever-engine)
(Copyright (c) 2026 MythicSim contributors, Copyright (c) 2022 wowsims team), linked into the
binary at the commit in `ENGINE_REF`. From that repository this one also takes:

- the item and spell data and the rotation presets, embedded at build time;
- `defaults/` and `testdata/gear/`, which started as its reference and healer requests
  (`validation/2026-10-04-production-buff-toggles/requests`, `fixtures/healers`) and have been
  edited here since.

## wowsims Forever

The talent calculator trees and the item and spell icons in the data bundle come from
[wowsims/forever](https://github.com/wowsims/forever) (Copyright (c) 2022 wowsims team) at the
commit in `UIDATA_REF`.

## AllTheThings

Where each item comes from (`sources.json`: instance bosses, quests, vendors, reputation, professions,
PvP and world drops) is read from the WoW Forever database of
[AllTheThings](https://github.com/ATTWoWAddon/AllTheThings) (Copyright (c) 2026 AllTheThings WoW Addon, MIT) at
the commit in `ATT_REF`.

## Client data

Trait ids, icon file ids and item quality are read from the game's own tables through
[wago.tools](https://wago.tools). World of Warcraft is a trademark of Blizzard Entertainment;
this project is not affiliated with Blizzard, MythicSim, the wowsims team or AllTheThings.
