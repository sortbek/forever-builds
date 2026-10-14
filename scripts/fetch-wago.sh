#!/usr/bin/env bash
# Downloads the trait and spec tables for the client build the engine data was generated from.
set -euo pipefail
cd "$(dirname "$0")/.."
BUILD=$(tr -d '[:space:]' < engine/assets/db_inputs/forever_client_build.txt)
mkdir -p wago
for t in TraitNode TraitNodeEntry TraitNodeXTraitNodeEntry ChrSpecialization SpellName; do
  curl -fsS -A "simhammer-forever-builds" -o "wago/$t.csv" "https://wago.tools/db2/$t/csv?build=$BUILD"
done
echo "$BUILD" > wago/BUILD
