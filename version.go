package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version      = "dev"
	EngineCommit = "unknown"
	ClientBuild  = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version, engine commit and client build as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := json.Marshal(map[string]string{
				"version": Version, "engine_commit": EngineCommit, "client_build": ClientBuild,
			})
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		},
	}
}
