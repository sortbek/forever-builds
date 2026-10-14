package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wowsims/forever/sim/core"
	"github.com/wowsims/forever/sim/core/proto"
)

func runSimRequest(req *proto.RaidSimRequest) (*proto.RaidSimResult, error) {
	if err := applyDefaults(req); err != nil {
		return nil, err
	}
	progress := make(chan *proto.ProgressMetrics, 10)
	core.RunRaidSimConcurrentAsync(req, progress, "simhammer-sim")
	for p := range progress {
		if p.FinalRaidResult != nil {
			if p.FinalRaidResult.Error != nil {
				return nil, fmt.Errorf("sim failed: %s", p.FinalRaidResult.Error.Message)
			}
			return p.FinalRaidResult, nil
		}
		reportProgress(p.CompletedIterations, p.TotalIterations)
	}
	return nil, errors.New("sim ended without a result")
}

func runSim(in, out string) error {
	req := &proto.RaidSimRequest{}
	if err := readMessage(in, req); err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	res, err := runSimRequest(req)
	if err != nil {
		return err
	}
	return writeMessage(out, res)
}

func newSimCmd() *cobra.Command {
	var in, out string
	cmd := &cobra.Command{
		Use:   "sim",
		Short: "Run one sim (RaidSimRequest in, RaidSimResult out)",
		RunE:  func(cmd *cobra.Command, args []string) error { return runSim(in, out) },
	}
	cmd.Flags().StringVar(&in, "in", "", "request JSON")
	cmd.Flags().StringVar(&out, "out", "", "result JSON")
	_ = cmd.MarkFlagRequired("in")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}
