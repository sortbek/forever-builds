#!/usr/bin/env bash
# Usage: package.sh <goos> <goarch> <asset-name>
set -euo pipefail
cd "$(dirname "$0")/.."
GOOS=$1 GOARCH=$2 NAME=$3
COMMIT=$(git -C engine rev-parse --short HEAD)
BUILD=$(cat wago/BUILD)
BIN=simhammer-forever; [ "$GOOS" = windows ] && BIN=simhammer-forever.exe
STAGE=dist/stage-$GOOS-$GOARCH
rm -rf "$STAGE" && mkdir -p "$STAGE"
CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -tags with_db \
  -ldflags "-s -w -X main.Version=$(date -u +%Y%m%d) -X main.EngineCommit=$COMMIT -X main.ClientBuild=$BUILD" \
  -o "$STAGE/$BIN" .
cp -r dist/data "$STAGE/data"
chmod 755 "$STAGE/$BIN"
if [ "$GOOS" = windows ]; then (cd "$STAGE" && zip -qr "../$NAME" .); else tar -czf "dist/$NAME" -C "$STAGE" .; fi
