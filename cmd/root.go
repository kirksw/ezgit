package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "ezgit",
	Short:         "An easy GitHub repository management CLI tool",
	Long:          `ezgit helps manage GitHub repositories with support for cloning, bare conversions, worktrees, and caching.`,
	SilenceErrors: true,
}

var (
	verbose    bool
	configPath string
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path (default: ./config.toml, ~/.config/ezgit/config.toml, or ~/.ezgit.toml)")
}
