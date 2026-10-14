package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func readMessage(path string, m proto.Message) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, m)
}

func writeMessage(path string, m proto.Message) error {
	data, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// SimHammer reads these lines from stderr; anything else on stderr is shown as log output.
func reportProgress(done, total int32) {
	fmt.Fprintf(os.Stderr, "progress %d %d\n", done, total)
}
