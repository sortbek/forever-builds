#!/usr/bin/env bash
# Checks out the engine at ENGINE_REF, generates its Go protos and copies APLs and talent trees.
set -euo pipefail
cd "$(dirname "$0")/.."
REF=$(tr -d '[:space:]' < ENGINE_REF)
if [ ! -d engine/.git ]; then
  git init -q engine
  git -C engine remote add origin https://github.com/ElliotWood/Forever.git
fi
git -C engine fetch -q --depth 1 origin "$REF"
git -C engine checkout -q FETCH_HEAD
protoc -I=engine/proto \
  --go_opt=Mgoogle/protobuf/descriptor.proto=google.golang.org/protobuf/types/descriptorpb \
  --go_out=engine/sim/core engine/proto/*.proto
rm -rf assets/apl assets/trees && mkdir -p assets/apl assets/trees
(cd engine/ui/specs && find . -path '*/apls/*.apl.json') | while read -r f; do
  dest="assets/apl/$(echo "$f" | sed 's#^\./##; s#/apls/#/#')"
  mkdir -p "$(dirname "$dest")" && cp "engine/ui/specs/$f" "$dest"
done
cp engine/ui/sim/talents/trees/*.json assets/trees/
if [ -f scripts/extract-defaults.mjs ]; then node scripts/extract-defaults.mjs engine assets/defaults rotation-lock.json; fi
echo "engine $(git -C engine rev-parse --short HEAD), client build $(tr -d '[:space:]' < engine/assets/db_inputs/forever_client_build.txt)"
