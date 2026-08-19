package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/ui"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <repo> [worktree-name]",
	Short: "Open a locally cloned repository with the configured open command",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runOpen,
}

func init() {
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

		repoPath := getRepoPath(cfg, repoFullName, false, repo.DefaultBranch)
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

	repoFullName, ok := extractRepoFullName(repoInput)
	if !ok {
		return fmt.Errorf("invalid repo format: %s", repoInput)
	}

	worktreeName = strings.TrimSpace(worktreeName)
	if worktreeName != "" {
		if err := ensureOpenWorktree(cfg, repoFullName, worktreeName); err != nil {
			return err
		}
		return runOpenCommand(cfg, repoFullName, worktreeName)
	}

	defaultBranch := resolveDefaultBranch(repoFullName, "")
	repoRootPath := getRepoPath(cfg, repoFullName, false, defaultBranch)
	if repoRootPath == "" {
		return fmt.Errorf("failed to resolve local path for %s", repoFullName)
	}

	if _, err := os.Stat(repoRootPath); os.IsNotExist(err) {
		originalWorktree := worktree
		worktree = false
		cloneErr := runDirectClone(cfg, repoFullName, defaultBranch, 0)
		worktree = originalWorktree
		if cloneErr != nil {
			return cloneErr
		}
	}

	return runOpenCommand(cfg, repoFullName, "")
}

func ensureOpenWorktree(cfg *config.Config, repoFullName, worktreeName string) error {
	defaultBranch := resolveDefaultBranch(repoFullName, "")
	repoRootPath := getRepoPath(cfg, repoFullName, false, defaultBranch)
	if repoRootPath == "" {
		return fmt.Errorf("failed to resolve local path for %s", repoFullName)
	}

	state, err := detectExistingRepoState(repoRootPath)
	if err != nil {
		return err
	}

	switch state {
	case existingRepoMissing:
		return cloneRepoWithWorktrees(cfg, repoFullName, defaultBranch, worktreeName)
	case existingRepoRegular:
		if err := runConvertPath(repoRootPath, defaultBranch); err != nil {
			return err
		}
	case existingRepoNonRepo:
		return fmt.Errorf("destination exists but is not a git repository: %s", repoRootPath)
	}

	return ensureWorktreeExists(git.New(), repoRootPath, repoFullName, defaultBranch, worktreeName)
}

func cloneRepoWithWorktrees(cfg *config.Config, repoFullName, defaultBranch, worktreeName string) error {
	planValue := cliCloneWorktreePlanFor(defaultBranch, worktreeName)
	plan := &planValue

	originalWorktree := worktree
	originalSkipPrompt := skipWorktreePrompt
	originalForcedPlan := forcedClonePlan
	worktree = true
	skipWorktreePrompt = true
	forcedClonePlan = plan
	defer func() {
		worktree = originalWorktree
		skipWorktreePrompt = originalSkipPrompt
		forcedClonePlan = originalForcedPlan
	}()

	return runDirectClone(cfg, repoFullName, defaultBranch, 0)
}

func ensureWorktreeExists(gitMgr git.GitManager, repoRootPath, repoFullName, defaultBranch, worktreeName string) error {
	worktrees, err := gitMgr.ListWorktrees(repoRootPath)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}
	if containsString(worktrees, worktreeName) {
		return nil
	}

	metadataPath := filepath.Join(repoRootPath, ".git")
	worktreePath := filepath.Join(repoRootPath, worktreeName)
	switch {
	case worktreeName == defaultBranch:
		return gitMgr.CreateWorktree(metadataPath, worktreePath, defaultBranch)
	case worktreeName == "review":
		return gitMgr.CreateDetachedWorktree(metadataPath, worktreePath, defaultBranch)
	default:
		return addWorktreeToRepo(gitMgr, repoFullName, repoRootPath, metadataPath, worktreeName)
	}
}

func isBuiltInWorktree(worktreeName, defaultBranch string) bool {
	return worktreeName == defaultBranch || worktreeName == "review"
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func runOpenRepoSelection(cfg *config.Config, repo *github.Repo, selectedWorktree string) error {
	if repo == nil {
		return nil
	}

	return runOpenCommand(cfg, repo.FullName, selectedWorktree)
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
		branchName := resolveDefaultBranch(repoFullName, defaultBranch)
		repoPath = filepath.Join(repoPath, branchName)
	}

	return repoPath
}
