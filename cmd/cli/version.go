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
		fmt.Println(buildVersion(""))
	},
}

func buildVersion(indent string) string {
	return fmt.Sprintf(
		"%sversion: %s\n%scommit: %s\n%sbuilt at: %s\n%sbuilt by: %s",
		indent, build.Version,
		indent, build.Commit,
		indent, build.Date,
		indent, build.BuiltBy,
	)
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
