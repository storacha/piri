package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/build"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of piri",
	Long:  `Print the version of piri including the git revision.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("version: %s\n", build.Version)
		fmt.Printf("commit: %s\n", build.Commit)
		fmt.Printf("built at: %s\n", build.Date)
		fmt.Printf("built by: %s\n", build.BuiltBy)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
