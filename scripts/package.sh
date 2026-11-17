#!/usr/bin/env bash
# Usage: package.sh <rust target> <asset-name>   (the binary must be built for the target already)
set -euo pipefail
cd "$(dirname "$0")/.."
TARGET=$1 NAME=$2
BIN=simhammer-forever; case "$TARGET" in *windows*) BIN=simhammer-forever.exe ;; esac
STAGE=dist/stage-$TARGET
rm -rf "$STAGE" && mkdir -p "$STAGE"
cp "target/$TARGET/release/$BIN" "$STAGE/$BIN"
cp -r dist/data "$STAGE/data"
cp LICENSE NOTICE.md "$STAGE/"
chmod 755 "$STAGE/$BIN"
case "$NAME" in
  *.zip) (cd "$STAGE" && 7z a -tzip -bso0 "../$NAME" .) ;;
  *) tar -czf "dist/$NAME" -C "$STAGE" . ;;
esac
