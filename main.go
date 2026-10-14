package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wowsims/forever/sim"
)

func main() {
	sim.RegisterAll()
	root := &cobra.Command{Use: "simhammer-forever", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newVersionCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
