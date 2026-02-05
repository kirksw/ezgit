package cmd

import (
	"fmt"
	"strings"

	"github.com/kirksw/ezgit/internal/git"
	"github.com/kirksw/ezgit/internal/utils"
	"github.com/spf13/cobra"
)

var bareConvertCmd = &cobra.Command{
	Use:   "bare-convert <path>",
	Short: "Convert an existing repository to bare with worktrees",
	Args:  cobra.ExactArgs(1),
	RunE:  runBareConvert,
}

var (
	worktrees      []string
	allWorktrees   bool
	noWorktrees    bool
	convertKeyPath string
)

func init() {
	rootCmd.AddCommand(bareConvertCmd)

	bareConvertCmd.Flags().StringSliceVarP(&worktrees, "worktree", "w", []string{}, "create worktree for specific branch")
	bareConvertCmd.Flags().BoolVar(&allWorktrees, "all-worktrees", false, "create worktree for all branches")
	bareConvertCmd.Flags().BoolVar(&noWorktrees, "no-worktrees", false, "skip worktree creation")
	bareConvertCmd.Flags().StringVar(&convertKeyPath, "key-path", "", "SSH key path")
}

func runBareConvert(cmd *cobra.Command, args []string) error {
	repoPath := args[0]

	gitMgr := git.New()

	if err := utils.ValidatePath(repoPath); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	fmt.Printf("Converting %s to bare repository...\n", repoPath)

	if err := gitMgr.ConvertToBare(repoPath); err != nil {
		return fmt.Errorf("failed to convert to bare: %w", err)
	}

	fmt.Println("✓ Successfully converted to bare repository")

	if noWorktrees {
		fmt.Println("Worktree creation skipped (--no-worktrees)")
		return nil
	}

	branches, err := gitMgr.ListBranches(repoPath)
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	var selectedBranches []string
	if allWorktrees {
		selectedBranches = branches
		fmt.Printf("Creating worktrees for all %d branches...\n", len(branches))
	} else if len(worktrees) > 0 {
		selectedBranches = worktrees
		fmt.Printf("Creating worktrees for %d branches...\n", len(selectedBranches))
	} else {
		fmt.Println("No worktrees specified, use --worktree <branch> or --all-worktrees to create worktrees")
		return nil
	}

	if err := utils.ValidateBranches(branches, selectedBranches); err != nil {
		return fmt.Errorf("branch validation failed: %w", err)
	}

	for _, branch := range selectedBranches {
		cleanBranch := strings.TrimSpace(strings.TrimPrefix(branch, "origin/"))
		worktreePath := utils.ParseWorktreePath(repoPath, cleanBranch)

		fmt.Printf("Creating worktree for %s at %s...\n", cleanBranch, worktreePath)

		if err := gitMgr.CreateWorktree(repoPath, worktreePath, cleanBranch); err != nil {
			fmt.Printf("Warning: failed to create worktree for %s: %v\n", cleanBranch, err)
			continue
		}

		fmt.Printf("✓ Worktree created: %s\n", worktreePath)
	}

	fmt.Println("\n✓ Bare repository conversion complete!")
	return nil
}
