#!/usr/bin/env bash
# Downloads the client tables of the newest live Forever build (wago product "wowf").
set -euo pipefail
cd "$(dirname "$0")/.."
UA="simhammer-forever-builds"
BUILD=${1:-$(curl -fsS -A "$UA" https://wago.tools/api/builds | node -e '
  const builds = JSON.parse(require("fs").readFileSync(0, "utf8")).wowf;
  const n = v => v.split(".").map(Number);
  builds.sort((a, b) => { const x = n(a.version), y = n(b.version); for (let i = 0; i < 4; i++) if (x[i] !== y[i]) return y[i] - x[i]; return 0; });
  console.log(builds[0].version);')}
mkdir -p wago
for t in TraitNode TraitNodeEntry TraitNodeXTraitNodeEntry ChrSpecialization SpellName Item ItemSparse ItemModifiedAppearance ItemAppearance; do
  curl -fsS -A "$UA" -o "wago/$t.csv" "https://wago.tools/db2/$t/csv?build=$BUILD"
done
echo "$BUILD" > wago/BUILD
echo "client build $BUILD"
