module github.com/sortbek/forever-builds

go 1.27.0

require (
	github.com/spf13/cobra v1.10.2
	github.com/wowsims/forever v0.0.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.1.0 // indirect
)

replace github.com/wowsims/forever => ./engine
