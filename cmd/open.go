package cmd

import (
	"fmt"
	"os"
	"os/exec"
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

const createNewWorktreeOption = "+ Create new worktree"

var openCmd = &cobra.Command{
	Use:   "open [repo] [worktree-name]",
	Short: "Open a locally cloned repository with the configured open command",
	Args:  cobra.MaximumNArgs(2),
	RunE:  runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

func runOpen(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		if len(args) != 2 {
			return fmt.Errorf("usage: ezgit open <repo> <worktree-name>")
		}
		return runOpenDirect(args[0], args[1])
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cloneDir := cfg.GetCloneDir()
	if cloneDir == "" {
		return fmt.Errorf("clone_dir must be set in config to use 'ezgit open'")
	}

	c := cache.New()
	if err := autoRefreshConfiguredCaches(cfg, c); err != nil {
		fmt.Printf("Warning: automatic cache refresh failed: %v\n", err)
	}

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

	result, err := ui.RunOpenFuzzySearch(localOnly, localRepos)
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}

	return runOpenRepoSelection(cfg, result.Repo, localOnly, localRepos, result.SelectedWorktree)
}

func runOpenDirect(repoInput string, worktreeName string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	repoFullName, ok := extractRepoFullName(repoInput)
	if !ok {
		return fmt.Errorf("invalid repo format: %s", repoInput)
	}

	worktreeName = strings.TrimSpace(worktreeName)
	if worktreeName == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	if err := runCloneWithWorktree(cfg, repoFullName, worktreeName); err != nil {
		return err
	}

	repoRootPath := getRepoPath(cfg, repoFullName, false, "")
	if repoRootPath == "" {
		return fmt.Errorf("failed to resolve local path for %s", repoFullName)
	}

	absPath := resolveOpenTargetPath(repoRootPath, worktreeName)
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("worktree path does not exist: %s", absPath)
	}

	return runSeshConnect(absPath)
}

func runSeshConnect(absPath string) error {
	cmd := exec.Command("sesh", "connect", absPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open session for %q: %w", absPath, err)
	}
	return nil
}

func runOpenRepoSelection(
	cfg *config.Config,
	repo *github.Repo,
	localOnly []github.Repo,
	localRepos map[string]bool,
	selectedWorktree string,
) error {
	if repo == nil {
		return nil
	}

	repoPath := getRepoPath(cfg, repo.FullName, false, repo.DefaultBranch)

	gitMgr := git.New()
	selectedWorktree, cancelled, err := selectOrCreateWorktreeForOpen(gitMgr, repoPath, repo, localOnly, localRepos, selectedWorktree)
	if err != nil {
		return err
	}
	if cancelled {
		return nil
	}

	return runOpenCommand(cfg, repo.FullName, selectedWorktree)
}

func selectOrCreateWorktreeForOpen(
	gitMgr git.GitManager,
	repoPath string,
	repo *github.Repo,
	localOnly []github.Repo,
	localRepos map[string]bool,
	selectedWorktree string,
) (string, bool, error) {
	if strings.TrimSpace(selectedWorktree) != "" {
		return selectedWorktree, false, nil
	}

	hasWorktrees, err := gitMgr.HasWorktrees(repoPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to check for worktrees: %w", err)
	}
	if !hasWorktrees {
		return "", false, nil
	}

	worktrees, err := gitMgr.ListWorktrees(repoPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to list worktrees: %w", err)
	}
	selectionOptions := withCreateWorktreeOption(worktrees)
	for {
		result, err := ui.RunWorktreeSelection(localOnly, repo, false, localRepos, selectionOptions)
		if err != nil {
			return "", false, err
		}
		if result.Repo == nil {
			return "", true, nil
		}
		if !isCreateWorktreeOption(result.SelectedWorktree) {
			return result.SelectedWorktree, false, nil
		}

		branches, err := gitMgr.ListBranches(repoPath)
		if err != nil {
			return "", false, fmt.Errorf("failed to list branches for new worktree: %w", err)
		}

		defaultBranch := strings.TrimSpace(repo.DefaultBranch)
		if defaultBranch == "" {
			defaultBranch = "main"
		}

		featureBranch, baseBranch, cancelled, err := ui.RunCreateWorktreePrompt(branches, defaultBranch)
		if err != nil {
			return "", false, fmt.Errorf("failed to configure new worktree: %w", err)
		}
		if cancelled {
			return "", true, nil
		}

		featureBranch = strings.TrimSpace(featureBranch)
		baseBranch = strings.TrimSpace(baseBranch)
		if featureBranch == "" {
			continue
		}
		if containsString(worktrees, featureBranch) {
			return "", false, fmt.Errorf("worktree %q already exists", featureBranch)
		}
		if baseBranch == "" {
			baseBranch = defaultBranch
		}

		worktreePath := filepath.Join(repoPath, featureBranch)
		if err := gitMgr.CreateFeatureWorktree(repoPath, worktreePath, featureBranch, baseBranch); err != nil {
			return "", false, fmt.Errorf("failed to create worktree %q from %q: %w", featureBranch, baseBranch, err)
		}

		return featureBranch, false, nil
	}
}

func withCreateWorktreeOption(worktrees []string) []string {
	options := make([]string, 0, len(worktrees)+1)
	options = append(options, worktrees...)
	options = append(options, createNewWorktreeOption)
	return options
}

func isCreateWorktreeOption(value string) bool {
	return strings.TrimSpace(value) == createNewWorktreeOption
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
