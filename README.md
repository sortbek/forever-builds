# forever-builds

`simhammer-forever` is the command-line runner SimHammer uses to sim WoW Forever characters. It wraps the
[WoW Forever Simulator](https://github.com/ElliotWood/Forever) engine (MIT, a fork of wowsims/classic) and applies the
same defaults the engine's own site applies, so SimHammer only sends the character, gear, talents and fight settings.

Build: `bash scripts/sync-engine.sh && go build -tags with_db .` (needs Go, protoc, protoc-gen-go and Node). CI builds
and publishes a release for every platform daily, after the tests and a smoke sim of every spec pass.
