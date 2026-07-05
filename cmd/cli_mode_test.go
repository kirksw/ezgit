package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestTUIOnlyRootAllowsNoArgs(t *testing.T) {
	if err := rootCmd.Args(rootCmd, nil); err != nil {
		t.Fatalf("root no args rejected: %v", err)
	}
	for _, cmd := range []*cobra.Command{cloneCmd, openCmd, convertCmd} {
		if err := cmd.Args(cmd, nil); err == nil {
			t.Fatalf("%s no args accepted; only bare ezgit should open the TUI", cmd.Use)
		}
	}
}

func TestRootVersionFlagUsesShortV(t *testing.T) {
	flag := rootCmd.Flags().Lookup("version")
	if flag == nil || flag.Shorthand != "v" {
		t.Fatalf("version flag = %#v, want shorthand -v", flag)
	}
	if rootCmd.PersistentFlags().Lookup("verbose").Shorthand != "" {
		t.Fatal("verbose should not keep -v shorthand")
	}
}
