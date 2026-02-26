package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/ui"
	"github.com/kirksw/ezgit/internal/utils"
)

func runRootDirect(cfg *config.Config, repoInput string, worktreeName string) error {
	repoFullName, ok := extractRepoFullName(repoInput)
	if !ok {
		return fmt.Errorf("invalid repo format: %s", repoInput)
	}

	worktreeName = strings.TrimSpace(worktreeName)
	if worktreeName != "" {
		if err := runCloneWithWorktree(cfg, repoFullName, worktreeName); err != nil {
			return err
		}
		if noOpen {
			return nil
		}
		return runOpenCommand(cfg, repoFullName, worktreeName)
	}

	defaultBranch := resolveDefaultBranch(repoFullName, "")
	repoPath := getRepoPath(cfg, repoFullName, false, defaultBranch)
	localExists := false
	if repoPath != "" {
		if _, err := os.Stat(repoPath); err == nil {
			localExists = true
		}
	}

	if !localExists {
		originalWorktree := worktree
		originalSkipPrompt := skipWorktreePrompt
		originalForcedPlan := forcedClonePlan
		worktree = false
		skipWorktreePrompt = false
		forcedClonePlan = nil
		err := runDirectClone(cfg, repoFullName, defaultBranch, 0)
		worktree = originalWorktree
		skipWorktreePrompt = originalSkipPrompt
		forcedClonePlan = originalForcedPlan
		if err != nil {
			return err
		}
	}

	if noOpen {
		return nil
	}

	repo := &github.Repo{FullName: repoFullName, DefaultBranch: defaultBranch}
	return runOpenRepoSelection(cfg, repo, []github.Repo{*repo}, map[string]bool{repoFullName: true}, "")
}

func runRootFuzzy(cfg *config.Config) error {
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

	localRepos := utils.BuildLocalRepoMap(cfg.GetCloneDir(), allRepos)
	result, err := ui.RunOpenFuzzySearch(allRepos, localRepos)
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}
	if result == nil || result.Repo == nil {
		return nil
	}

	repo := result.Repo
	repoPath := getRepoPath(cfg, repo.FullName, false, repo.DefaultBranch)
	localExists := false
	if repoPath != "" {
		if _, err := os.Stat(repoPath); err == nil {
			localExists = true
		}
	}

	if !localExists {
		defaultBranch := resolveDefaultBranch(repo.FullName, repo.DefaultBranch)
		shouldUseWorktrees := false
		selectedPlan := cloneWorktreePlan{}

		if isInteractiveStdin() {
			plan, cancelled, planErr := ui.RunCloneWorktreePlanPrompt([]string{defaultBranch}, defaultBranch, false)
			if planErr != nil {
				return fmt.Errorf("failed to configure clone options: %w", planErr)
			}
			if cancelled {
				return nil
			}

			selectedCount := 0
			if plan.CreateDefault {
				selectedCount++
			}
			if plan.CreateReview {
				selectedCount++
			}
			selectedCount += len(plan.Custom)
			shouldUseWorktrees = selectedCount > 1

			if shouldUseWorktrees {
				selectedPlan.CreateDefault = plan.CreateDefault
				selectedPlan.CreateReview = plan.CreateReview
				for _, custom := range plan.Custom {
					selectedPlan.Custom = append(selectedPlan.Custom, cloneCustomWorktree{
						Name:       custom.Name,
						BaseBranch: custom.BaseBranch,
					})
				}
			}
		}

		originalWorktree := worktree
		originalSkipPrompt := skipWorktreePrompt
		originalForcedPlan := forcedClonePlan
		originalFeature := featureWorktree
		originalFeatureBase := featureBaseBranch

		worktree = shouldUseWorktrees
		skipWorktreePrompt = shouldUseWorktrees
		featureWorktree = ""
		featureBaseBranch = ""
		if shouldUseWorktrees {
			forcedClonePlan = &selectedPlan
		} else {
			forcedClonePlan = nil
		}

		cloneErr := runDirectClone(cfg, repo.FullName, repo.DefaultBranch, repo.Size)

		worktree = originalWorktree
		skipWorktreePrompt = originalSkipPrompt
		forcedClonePlan = originalForcedPlan
		featureWorktree = originalFeature
		featureBaseBranch = originalFeatureBase

		if cloneErr != nil {
			return fmt.Errorf("failed to clone repo: %w", cloneErr)
		}

		localRepos[repo.FullName] = true
	}

	if noOpen {
		return nil
	}

	localOnlyRepos := make([]github.Repo, 0, len(allRepos))
	for _, r := range allRepos {
		if localRepos[r.FullName] {
			localOnlyRepos = append(localOnlyRepos, r)
		}
	}

	return runOpenRepoSelection(cfg, repo, localOnlyRepos, localRepos, result.SelectedWorktree)
}
