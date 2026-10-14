package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wowsims/forever/sim"
)

// Registers every spec's agent factory; in init so the tests get it too.
func init() {
	sim.RegisterAll()
}

func main() {
	root := &cobra.Command{Use: "simhammer-forever", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newVersionCmd(), newSimCmd(), newBulkCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
