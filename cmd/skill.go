package cmd

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed ezgit-skill.md
var ezgitSkill string

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print the ezgit agent skill",
	Args:  cobra.NoArgs,
	RunE:  runSkill,
}

func init() {
	rootCmd.AddCommand(skillCmd)
}

func runSkill(cmd *cobra.Command, args []string) error {
	_, err := fmt.Fprint(cmd.OutOrStdout(), ezgitSkill)
	return err
}
