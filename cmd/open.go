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

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open a locally cloned repository in sesh",
	RunE:  runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

func runOpen(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cloneDir := cfg.GetCloneDir()
	if cloneDir == "" {
		return fmt.Errorf("clone_dir must be set in config to use 'ezgit open'")
	}

	c := cache.New()
	allRepos, err := c.GetAllRepos()
	if err != nil {
		return fmt.Errorf("failed to load cached repos: %w", err)
	}

	if len(allRepos) == 0 {
		return fmt.Errorf("no cached repositories found. Run 'ezgit cache refresh' to fetch repos")
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

	result, err := ui.RunFuzzySearch(localOnly, false, localRepos, true)
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}

	repoPath := getRepoPath(cfg, result.Repo.FullName, false, result.Repo.DefaultBranch)

	gitMgr := git.New()
	hasWorktrees, err := gitMgr.HasWorktrees(repoPath)
	if err != nil {
		return fmt.Errorf("failed to check for worktrees: %w", err)
	}

	if hasWorktrees && result.SelectedWorktree == "" {
		worktrees, err := gitMgr.ListWorktrees(repoPath)
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		result, err = ui.RunWorktreeSelection(localOnly, result.Repo, false, localRepos, worktrees)
		if err != nil {
			return err
		}

		if result.Repo == nil {
			return nil
		}
	}

	return runSeshConnect(cfg, result.Repo.FullName, result.Repo.DefaultBranch, false, result.SelectedWorktree)
}

func getRepoPath(cfg *config.Config, repoFullName string, isWorktree bool, defaultBranch string) string {
	cloneDir := cfg.GetCloneDir()
	if cloneDir == "" {
		return ""
	}

	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return ""
	}

	owner, repoName := parts[0], parts[1]
	repoPath := filepath.Join(cloneDir, owner, repoName)

	if isWorktree {
		branchName := defaultBranch
		if branchName == "" {
			c := cache.New()
			allRepos, err := c.GetAllRepos()
			if err == nil {
				for _, r := range allRepos {
					if r.FullName == repoFullName {
						branchName = r.DefaultBranch
						break
					}
				}
			}
		}
		repoPath = filepath.Join(repoPath, branchName)
	}

	return repoPath
}
