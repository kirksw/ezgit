package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/ui"
	"github.com/kirksw/ezgit/internal/utils"
	"github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
	Use:   "convert [path]",
	Short: "Convert an existing repository to bare with worktrees",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConvert,
}

var (
	worktrees      []string
	allWorktrees   bool
	noWorktrees    bool
	convertKeyPath string
)

func init() {
	rootCmd.AddCommand(convertCmd)

	convertCmd.Flags().StringSliceVarP(&worktrees, "worktree", "w", []string{}, "create worktree for specific branch")
	convertCmd.Flags().BoolVar(&allWorktrees, "all-worktrees", false, "create worktree for all branches")
	convertCmd.Flags().BoolVar(&noWorktrees, "no-worktrees", false, "skip worktree creation")
	convertCmd.Flags().StringVar(&convertKeyPath, "key-path", "", "SSH key path")
}

func runConvert(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return runConvertFromLocalRepoPicker()
	}

	return runConvertPath(args[0], "")
}

func runConvertFromLocalRepoPicker() error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cloneDir := cfg.GetCloneDir()
	if cloneDir == "" {
		return fmt.Errorf("clone_dir must be set in config to use 'ezgit convert' without a path")
	}

	c := cache.New()
	if err := autoRefreshConfiguredCaches(cfg, c); err != nil {
		fmt.Printf("Warning: automatic cache refresh failed: %v\n", err)
	}

	allRepos, err := c.GetAllRepos()
	if err != nil {
		return fmt.Errorf("failed to load cached repos: %w", err)
	}

	localRepos := utils.BuildLocalRepoMap(cloneDir, allRepos)
	var localOnly []github.Repo
	for _, repo := range allRepos {
		if localRepos[repo.FullName] {
			localOnly = append(localOnly, repo)
		}
	}
	if len(localOnly) == 0 {
		return fmt.Errorf("no locally cloned repositories found in %s", cloneDir)
	}

	result, err := ui.RunOpenFuzzySearch(localOnly, localRepos)
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}

	repoPath := getRepoPath(cfg, result.Repo.FullName, false, result.Repo.DefaultBranch)
	return runConvertPath(repoPath, result.Repo.DefaultBranch)
}

func runConvertPath(repoPath, repoDefaultBranch string) error {
	gitMgr := git.New()

	if err := utils.ValidatePath(repoPath); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	repoPath = absRepoPath

	createDefaultWorktree := true
	createReviewWorktree := true
	featureBranch := ""
	baseBranch := ""
	defaultBranch := ""

	useDefaultWorktreeFlow := !noWorktrees && !allWorktrees && len(worktrees) == 0
	if useDefaultWorktreeFlow {
		branches, err := gitMgr.ListBranches(repoPath)
		if err != nil {
			return fmt.Errorf("failed to list branches: %w", err)
		}
		defaultBranch = resolveConvertDefaultBranch(repoDefaultBranch, branches)

		if isInteractiveStdin() {
			requestCustomWorktree := false
			var cancelled bool
			createDefaultWorktree, createReviewWorktree, requestCustomWorktree, cancelled, err = ui.RunCloneWorktreeOptionsPrompt(defaultBranch)
			if err != nil {
				return fmt.Errorf("failed to configure convert worktree options: %w", err)
			}
			if cancelled {
				return fmt.Errorf("cancelled")
			}

			if requestCustomWorktree {
				featureBranch, baseBranch, cancelled, err = ui.RunCreateWorktreePrompt(branches, defaultBranch)
				if err != nil {
					return fmt.Errorf("failed to configure custom worktree: %w", err)
				}
				if cancelled {
					return fmt.Errorf("cancelled")
				}
				featureBranch, baseBranch, err = resolveFeatureWorktreeConfig(defaultBranch, featureBranch, baseBranch)
				if err != nil {
					return err
				}
			}
		}
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

	bareMetadataPath := filepath.Join(repoPath, ".git")

	branches, err := gitMgr.ListBranches(bareMetadataPath)
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}
	if defaultBranch == "" {
		defaultBranch = resolveConvertDefaultBranch(repoDefaultBranch, branches)
	}

	switch {
	case allWorktrees:
		if err := createAllConvertWorktrees(gitMgr, bareMetadataPath, repoPath, branches); err != nil {
			return err
		}
	case len(worktrees) > 0:
		if err := createSelectedConvertWorktrees(gitMgr, bareMetadataPath, repoPath, branches, worktrees); err != nil {
			return err
		}
	default:
		if err := createPlannedConvertWorktrees(
			gitMgr,
			bareMetadataPath,
			repoPath,
			defaultBranch,
			createDefaultWorktree,
			createReviewWorktree,
			featureBranch,
			baseBranch,
		); err != nil {
			return err
		}
	}

	fmt.Println("\n✓ Bare repository conversion complete!")
	return nil
}

func createAllConvertWorktrees(gitMgr git.GitManager, bareMetadataPath, repoPath string, branches []string) error {
	fmt.Printf("Creating worktrees for all %d branches...\n", len(branches))
	for _, branch := range branches {
		cleanBranch := strings.TrimSpace(strings.TrimPrefix(branch, "origin/"))
		worktreePath := utils.ParseWorktreePath(repoPath, cleanBranch)

		fmt.Printf("Creating worktree for %s at %s...\n", cleanBranch, worktreePath)
		if err := gitMgr.CreateWorktree(bareMetadataPath, worktreePath, cleanBranch); err != nil {
			fmt.Printf("Warning: failed to create worktree for %s: %v\n", cleanBranch, err)
			continue
		}
		fmt.Printf("✓ Worktree created: %s\n", worktreePath)
	}
	return nil
}

func createSelectedConvertWorktrees(gitMgr git.GitManager, bareMetadataPath, repoPath string, branches []string, selected []string) error {
	fmt.Printf("Creating worktrees for %d branches...\n", len(selected))
	if err := utils.ValidateBranches(branches, selected); err != nil {
		return fmt.Errorf("branch validation failed: %w", err)
	}
	for _, branch := range selected {
		cleanBranch := strings.TrimSpace(strings.TrimPrefix(branch, "origin/"))
		worktreePath := utils.ParseWorktreePath(repoPath, cleanBranch)
		fmt.Printf("Creating worktree for %s at %s...\n", cleanBranch, worktreePath)
		if err := gitMgr.CreateWorktree(bareMetadataPath, worktreePath, cleanBranch); err != nil {
			fmt.Printf("Warning: failed to create worktree for %s: %v\n", cleanBranch, err)
			continue
		}
		fmt.Printf("✓ Worktree created: %s\n", worktreePath)
	}
	return nil
}

func createPlannedConvertWorktrees(
	gitMgr git.GitManager,
	bareMetadataPath, repoPath, defaultBranch string,
	createDefaultWorktree bool,
	createReviewWorktree bool,
	featureBranch string,
	baseBranch string,
) error {

	if createDefaultWorktree {
		defaultWorktreePath := filepath.Join(repoPath, defaultBranch)
		fmt.Printf("Creating default worktree for %q at %s...\n", defaultBranch, defaultWorktreePath)
		if err := gitMgr.CreateWorktree(bareMetadataPath, defaultWorktreePath, defaultBranch); err != nil {
			return fmt.Errorf("failed to create default worktree for branch %s: %w", defaultBranch, err)
		}
		fmt.Printf("✓ Worktree created: %s\n", defaultWorktreePath)
	}

	if createReviewWorktree {
		reviewWorktreePath := filepath.Join(repoPath, "review")
		fmt.Printf("Creating review worktree from %q at %s...\n", defaultBranch, reviewWorktreePath)
		if err := gitMgr.CreateDetachedWorktree(bareMetadataPath, reviewWorktreePath, defaultBranch); err != nil {
			return fmt.Errorf("failed to create review worktree from branch %s: %w", defaultBranch, err)
		}
		fmt.Printf("✓ Worktree created: %s\n", reviewWorktreePath)
	}

	if featureBranch != "" {
		featureWorktreePath := filepath.Join(repoPath, featureBranch)
		fmt.Printf("Creating custom worktree %q from %q at %s...\n", featureBranch, baseBranch, featureWorktreePath)
		if err := gitMgr.CreateFeatureWorktree(bareMetadataPath, featureWorktreePath, featureBranch, baseBranch); err != nil {
			return fmt.Errorf("failed to create custom worktree %s from %s: %w", featureBranch, baseBranch, err)
		}
		fmt.Printf("✓ Worktree created: %s\n", featureWorktreePath)
	}

	return nil
}

func resolveConvertDefaultBranch(explicit string, branches []string) string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		for _, branch := range branches {
			if strings.TrimSpace(branch) == explicit {
				return explicit
			}
		}
	}

	for _, preferred := range []string{"main", "master"} {
		for _, branch := range branches {
			if strings.TrimSpace(branch) == preferred {
				return preferred
			}
		}
	}

	if len(branches) > 0 {
		candidate := strings.TrimSpace(branches[0])
		if candidate != "" {
			return candidate
		}
	}

	if explicit != "" {
		return explicit
	}
	return "main"
}
