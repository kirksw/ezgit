package cmd

import (
	"fmt"

	"github.com/kirksw/ezgit/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ezgit v%s\n", version.Value)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
