package benchmark

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run performance benchmarks",
	Long:  "Run various performance benchmarks for Piri components",
}

func init() {
	Cmd.AddCommand(commpCmd)
	Cmd.AddCommand(sha256Cmd)
}
