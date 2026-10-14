package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wowsims/forever/sim/core/proto"
)

func bulkRequest(t *testing.T) *proto.BulkSimRequest {
	base := specRequest(t, "retributionPaladin")
	base.SimOptions.Iterations = 200
	// Two extra main-hand options.
	return &proto.BulkSimRequest{
		BaseRequest:  base,
		BulkSettings: &proto.BulkSettings{Items: []*proto.ItemSpec{{Id: 12784}, {Id: 19019}}},
	}
}

func TestBulkCommandReturnsBaselineAndResults(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.json"), filepath.Join(dir, "out.json")
	if err := writeMessage(in, bulkRequest(t)); err != nil {
		t.Fatal(err)
	}
	if err := runBulk(in, out, 1000); err != nil {
		t.Fatal(err)
	}
	res := &proto.BulkSimResult{}
	if err := readMessage(out, res); err != nil {
		t.Fatal(err)
	}
	if res.Baseline.GetDpsMetrics().GetAvg() <= 0 || len(res.TopResults) == 0 {
		t.Errorf("baseline %v, %d results", res.Baseline, len(res.TopResults))
	}
}

func TestBulkCommandRefusesTooManyCombinations(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.json"), filepath.Join(dir, "out.json")
	if err := writeMessage(in, bulkRequest(t)); err != nil {
		t.Fatal(err)
	}
	err := runBulk(in, out, 1)
	if err == nil || !strings.Contains(err.Error(), "more than the limit of 1") {
		t.Errorf("err = %v", err)
	}
}
