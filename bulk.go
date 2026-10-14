package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wowsims/forever/sim/core/bulk"
	"github.com/wowsims/forever/sim/core/proto"
)

const defaultTopResults = 25

func runBulk(in, out string, maxCombinations int32) error {
	req := &proto.BulkSimRequest{}
	if err := readMessage(in, req); err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	if req.BaseRequest == nil {
		return errors.New("bulk request has no baseRequest")
	}
	if err := applyDefaults(req.BaseRequest); err != nil {
		return err
	}
	if req.BulkSettings == nil {
		req.BulkSettings = &proto.BulkSettings{}
	}
	count := bulk.BulkCombinationCount(&proto.BulkCombinationCountRequest{
		BaseRequest: req.BaseRequest, BulkSettings: req.BulkSettings,
	})
	if count.Error != nil {
		return fmt.Errorf("count combinations: %s", count.Error.Message)
	}
	if maxCombinations > 0 && count.Combinations > maxCombinations {
		return fmt.Errorf("%d combinations is more than the limit of %d. Select fewer items.", count.Combinations, maxCombinations)
	}
	if req.TopResults == 0 {
		req.TopResults = defaultTopResults
	}
	// The UI's rule: the iteration setting is the high stage's floor.
	if req.HighStageIterations == 0 {
		req.HighStageIterations = req.BaseRequest.SimOptions.Iterations
	}
	if err := bulk.EnsureBulkSimCandidatesGenerated(req); err != nil {
		return fmt.Errorf("build combinations: %w", err)
	}
	progress := make(chan *proto.ProgressMetrics, 10)
	bulk.BulkSimAsync(req, progress, "simhammer-bulk")
	var result *proto.BulkSimResult
	for p := range progress {
		if p.FinalBulkSimResult != nil {
			result = p.FinalBulkSimResult
			break
		}
		reportProgress(p.CompletedSims, p.TotalSims)
	}
	if result == nil {
		return errors.New("bulk sim ended without a result")
	}
	if result.Error != nil {
		return fmt.Errorf("bulk sim failed: %s", result.Error.Message)
	}
	return writeMessage(out, result)
}

func newBulkCmd() *cobra.Command {
	var in, out string
	var maxCombinations int32
	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Run a gear bulk sim (BulkSimRequest in, BulkSimResult out)",
		RunE:  func(cmd *cobra.Command, args []string) error { return runBulk(in, out, maxCombinations) },
	}
	cmd.Flags().StringVar(&in, "in", "", "request JSON")
	cmd.Flags().StringVar(&out, "out", "", "result JSON")
	cmd.Flags().Int32Var(&maxCombinations, "max-combinations", 0, "refuse when the request has more combinations (0: no limit)")
	_ = cmd.MarkFlagRequired("in")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}
