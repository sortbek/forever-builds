module github.com/sortbek/forever-builds

go 1.25.0

require (
	github.com/spf13/cobra v1.7.0
	github.com/wowsims/forever v0.0.0
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
)

replace github.com/wowsims/forever => ./engine
