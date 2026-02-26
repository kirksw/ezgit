package cmd

import (
	"fmt"
	"os"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "ezgit",
	Args:          cobra.MaximumNArgs(2),
	Short:         "An easy GitHub repository management CLI tool",
	Long:          `ezgit helps manage GitHub repositories with support for cloning, bare conversions, worktrees, and caching.`,
	SilenceErrors: true,
	RunE:          runRoot,
}

var (
	verbose    bool
	configPath string
	noOpen     bool
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
	rootCmd.Flags().BoolVar(&noOpen, "no-open", false, "prepare repository/worktree but do not run open command")
}

func runRoot(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch len(args) {
	case 0:
		return runRootFuzzy(cfg)
	case 1:
		return runRootDirect(cfg, args[0], "")
	case 2:
		return runRootDirect(cfg, args[0], args[1])
	default:
		return fmt.Errorf("usage: ezgit [owner/repo] [worktree]")
	}
}
