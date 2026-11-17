#!/usr/bin/env bash
# Checks out the sim engine at ENGINE_REF (engine/) and the UI data at UIDATA_REF (uidata/):
# the talent calculator trees and the item and spell icons come from the wowsims Forever UI.
set -euo pipefail
cd "$(dirname "$0")/.."
checkout() { # <dir> <url> <ref file>
  local ref; ref=$(tr -d '[:space:]' < "$3")
  [ -d "$1/.git" ] || git init -q "$1"
  git -C "$1" remote remove origin 2>/dev/null || true
  git -C "$1" remote add origin "$2"
  git -C "$1" fetch -q --depth 1 origin "$ref"
  git -C "$1" checkout -q FETCH_HEAD
  echo "$1 $(git -C "$1" rev-parse --short HEAD)"
}
checkout engine https://github.com/sage3648/mythicsim-forever-engine.git ENGINE_REF
checkout uidata https://github.com/wowsims/forever.git UIDATA_REF
