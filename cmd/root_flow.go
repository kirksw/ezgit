package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
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
	allRepos, err := c.GetAllRepos()
	if err != nil {
		return fmt.Errorf("failed to load cached repos: %w", err)
	}

	var backgroundRefreshDone <-chan error
	if len(allRepos) == 0 {
		if err := autoRefreshConfiguredCaches(cfg, c); err != nil {
			fmt.Printf("Warning: automatic cache refresh failed: %v\n", err)
		}

		allRepos, err = c.GetAllRepos()
		if err != nil {
			return fmt.Errorf("failed to load cached repos: %w", err)
		}
	} else {
		backgroundRefreshDone = startAutoRefreshConfiguredCaches(cfg, c)
		defer warnIfBackgroundAutoRefreshFailed(backgroundRefreshDone)
	}

	if len(allRepos) == 0 {
		return fmt.Errorf("no cached repositories found. Run 'ezgit cache refresh' to fetch repos")
	}

	seedDefaultBranchLookupFromRepos(allRepos)

	var (
		localRepos      map[string]bool
		openedRepos     map[string]bool
		openedWorktrees map[string]map[string]bool
	)

	var buildWg sync.WaitGroup
	buildWg.Add(2)

	go func() {
		defer buildWg.Done()
		localRepos = utils.BuildLocalRepoMap(cfg.GetCloneDir(), allRepos)
	}()

	go func() {
		defer buildWg.Done()
		fetchedSessions, sessionErr := listTmuxSessions()
		if sessionErr != nil {
			openedRepos = make(map[string]bool)
			openedWorktrees = make(map[string]map[string]bool)
			return
		}
		openedRepos, openedWorktrees = buildOpenedMapsFromSessions(allRepos, fetchedSessions)
	}()

	buildWg.Wait()

	worktreeLoader := func(repo github.Repo) ([]string, error) {
		if !localRepos[repo.FullName] {
			return nil, nil
		}

		repoPath := getRepoPath(cfg, repo.FullName, false, repo.DefaultBranch)
		if strings.TrimSpace(repoPath) == "" {
			return nil, nil
		}

		gitMgr := git.New()
		worktrees, err := gitMgr.ListWorktrees(repoPath)
		if err != nil {
			return nil, err
		}
		return worktrees, nil
	}

	result, err := ui.RunOpenFuzzySearchWithOpenedAndLoader(allRepos, localRepos, openedRepos, openedWorktrees, nil, worktreeLoader)
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}
	if result == nil || result.Repo == nil {
		return nil
	}

	repo := result.Repo
	if result.CreateWorktree {
		if err := runCloneWithWorktreeAndBase(cfg, repo.FullName, result.SelectedWorktree, result.WorktreeBase); err != nil {
			return err
		}
		if noOpen {
			return nil
		}
		return runOpenCommand(cfg, repo.FullName, result.SelectedWorktree)
	}

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
	return runOpenCommand(cfg, repo.FullName, result.SelectedWorktree)
}

func buildOpenedRepoMap(allRepos []github.Repo) map[string]bool {
	sessions, err := listTmuxSessions()
	if err != nil {
		return make(map[string]bool)
	}
	openedRepos, _ := buildOpenedMapsFromSessions(allRepos, sessions)
	return openedRepos
}

func buildOpenedRepoMapFromSessions(allRepos []github.Repo, sessions []string) map[string]bool {
	openedRepos, _ := buildOpenedMapsFromSessions(allRepos, sessions)
	return openedRepos
}

func buildOpenedWorktreeMap(allRepos []github.Repo) map[string]map[string]bool {
	sessions, err := listTmuxSessions()
	if err != nil {
		return make(map[string]map[string]bool)
	}
	_, openedWorktrees := buildOpenedMapsFromSessions(allRepos, sessions)
	return openedWorktrees
}

func buildOpenedWorktreeMapFromSessions(allRepos []github.Repo, sessions []string) map[string]map[string]bool {
	_, openedWorktrees := buildOpenedMapsFromSessions(allRepos, sessions)
	return openedWorktrees
}

type repoSessionMetadata struct {
	fullName string
	markers  []string
}

func buildOpenedMapsFromSessions(allRepos []github.Repo, sessions []string) (map[string]bool, map[string]map[string]bool) {
	openedRepos := make(map[string]bool)
	opened := make(map[string]map[string]bool)
	if len(sessions) == 0 {
		return openedRepos, opened
	}

	repoMetadata := make([]repoSessionMetadata, 0, len(allRepos))
	for _, repo := range allRepos {
		repoFullName := strings.TrimSpace(repo.FullName)
		if repoFullName == "" {
			continue
		}
		repoMetadata = append(repoMetadata, repoSessionMetadata{
			fullName: repoFullName,
			markers:  repoSessionMarkers(repoFullName),
		})
	}

	if len(repoMetadata) == 0 {
		return openedRepos, opened
	}

	for _, session := range sessions {
		session = strings.TrimSpace(session)
		if session == "" {
			continue
		}
		for _, repo := range repoMetadata {
			if sessionMatchesRepoWithMarkers(session, repo.markers) {
				openedRepos[repo.fullName] = true
			}

			worktree, ok := extractWorktreeFromSessionWithMarkers(session, repo.markers)
			if !ok {
				continue
			}
			if strings.TrimSpace(worktree) == "" {
				continue
			}
			if _, ok := opened[repo.fullName]; !ok {
				opened[repo.fullName] = make(map[string]bool)
			}
			opened[repo.fullName][worktree] = true
		}
	}

	return openedRepos, opened
}

const defaultWorktreeLookupConcurrency = 8

type repoWorktreeLister interface {
	ListWorktrees(path string) ([]string, error)
}

func buildLocalRepoWorktreeMap(cfg *config.Config, allRepos []github.Repo, localRepos map[string]bool) map[string][]string {
	return buildLocalRepoWorktreeMapWithLister(cfg, allRepos, localRepos, git.New(), defaultWorktreeLookupConcurrency)
}

func buildLocalRepoWorktreeMapWithLister(cfg *config.Config, allRepos []github.Repo, localRepos map[string]bool, lister repoWorktreeLister, workers int) map[string][]string {
	result := make(map[string][]string)
	if len(allRepos) == 0 || len(localRepos) == 0 || lister == nil {
		return result
	}
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan github.Repo)
	var wg sync.WaitGroup
	var resultMu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repo := range jobs {
				if !localRepos[repo.FullName] {
					continue
				}
				repoPath := getRepoPath(cfg, repo.FullName, false, repo.DefaultBranch)
				if strings.TrimSpace(repoPath) == "" {
					continue
				}
				worktrees, err := lister.ListWorktrees(repoPath)
				if err != nil || len(worktrees) == 0 {
					continue
				}

				resultMu.Lock()
				result[repo.FullName] = append([]string(nil), worktrees...)
				resultMu.Unlock()
			}
		}()
	}

	for _, repo := range allRepos {
		jobs <- repo
	}
	close(jobs)
	wg.Wait()

	return result
}

func sessionMatchesRepo(session string, repoFullName string) bool {
	session = strings.TrimSpace(session)
	repoFullName = strings.TrimSpace(repoFullName)
	if session == "" || repoFullName == "" {
		return false
	}

	return sessionMatchesRepoWithMarkers(session, repoSessionMarkers(repoFullName))
}

func sessionMatchesRepoWithMarkers(session string, markers []string) bool {
	if session == "" || len(markers) == 0 {
		return false
	}

	for _, marker := range markers {
		if session == marker || strings.HasPrefix(session, marker+"/") {
			return true
		}
		if strings.Contains(session, "/"+marker+"/") || strings.HasSuffix(session, "/"+marker) {
			return true
		}
	}

	return false
}

func extractWorktreeFromSession(session string, repoFullName string) (string, bool) {
	session = strings.TrimSpace(session)
	repoFullName = strings.TrimSpace(repoFullName)
	if session == "" || repoFullName == "" {
		return "", false
	}

	return extractWorktreeFromSessionWithMarkers(session, repoSessionMarkers(repoFullName))
}

func extractWorktreeFromSessionWithMarkers(session string, markers []string) (string, bool) {
	if session == "" || len(markers) == 0 {
		return "", false
	}

	for _, marker := range markers {
		prefix := marker + "/"
		if strings.HasPrefix(session, prefix) {
			wt := strings.TrimSpace(strings.TrimPrefix(session, prefix))
			return wt, wt != ""
		}

		idx := strings.Index(session, "/"+prefix)
		if idx >= 0 {
			start := idx + len("/"+prefix)
			if start < len(session) {
				wt := strings.TrimSpace(session[start:])
				return wt, wt != ""
			}
		}
	}

	return "", false
}

func repoSessionMarkers(repoFullName string) []string {
	repoFullName = strings.TrimSpace(repoFullName)
	if repoFullName == "" {
		return nil
	}
	normalized := strings.ReplaceAll(repoFullName, "/", "-")
	if normalized == repoFullName {
		return []string{repoFullName}
	}
	return []string{repoFullName, normalized}
}
