package cmd

import (
	"fmt"
	"strings"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/ui"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <repo> [worktree-name]",
	Short: "Open an existing repository or worktree",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runOpen,
}

func runOpen(cmd *cobra.Command, args []string) error {
	worktreeName := ""
	if len(args) == 2 {
		worktreeName = args[1]
	}
	return runOpenDirect(args[0], worktreeName)
}

func buildOpenWorktreeLoader(cfg *config.Config, localRepos map[string]bool, lister repoWorktreeLister) ui.RepoWorktreeLoader {
	if cfg == nil || lister == nil {
		return nil
	}

	return func(repo github.Repo) ([]string, error) {
		repoFullName := strings.TrimSpace(repo.FullName)
		if repoFullName == "" || !localRepos[repoFullName] {
			return nil, nil
		}

		repoPath := getRepoPath(cfg, repoFullName)
		if strings.TrimSpace(repoPath) == "" {
			return nil, nil
		}

		return lister.ListWorktrees(repoPath)
	}
}

func runOpenDirect(repoInput string, worktreeName string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	return runOpenCommand(cfg, repoInput, worktreeName)
}

func runOpenRepoSelection(cfg *config.Config, repo *github.Repo, selectedWorktree string) error {
	if repo == nil {
		return nil
	}

	return runOpenCommand(cfg, repo.FullName, selectedWorktree)
}

func getRepoPath(cfg *config.Config, repoInput string) string {
	repo, err := parseRepository(repoInput)
	if err != nil {
		return ""
	}
	return repo.repoPath(cfg)
}
