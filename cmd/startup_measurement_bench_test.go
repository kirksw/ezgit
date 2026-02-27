package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/utils"
)

type startupSummary struct {
	localRepoCount      int
	openedRepoCount     int
	openedWorktreeCount int
	initialWorktrees    int
}

type startupBenchmarkWorktreeLister struct {
	delay time.Duration
}

func (l startupBenchmarkWorktreeLister) ListWorktrees(path string) ([]string, error) {
	if l.delay > 0 {
		time.Sleep(l.delay)
	}
	return []string{"main", "review"}, nil
}

var startupSummarySink startupSummary

func BenchmarkFuzzyStartupPreparation(b *testing.B) {
	cloneDir := b.TempDir()
	cfg := &config.Config{Git: config.GitConfig{CloneDir: cloneDir}}

	repoCount := 250
	allRepos := make([]github.Repo, 0, repoCount)
	for i := 0; i < repoCount; i++ {
		fullName := fmt.Sprintf("acme/repo-%d", i)
		allRepos = append(allRepos, github.Repo{FullName: fullName, DefaultBranch: "main"})

		repoDir := filepath.Join(cloneDir, "acme", fmt.Sprintf("repo-%d", i))
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			b.Fatalf("mkdir %s: %v", repoDir, err)
		}
	}

	sessions := make([]string, 0, repoCount/5)
	for i := 0; i < repoCount; i += 5 {
		sessions = append(sessions, fmt.Sprintf("acme/repo-%d/main", i))
	}

	cases := []struct {
		name        string
		delay       time.Duration
		workerCount int
		concurrent  bool
	}{
		{name: "sequential_cpu", delay: 0, workerCount: 1, concurrent: false},
		{name: "concurrent_cpu", delay: 0, workerCount: 8, concurrent: true},
		{name: "sequential_io", delay: 500 * time.Microsecond, workerCount: 1, concurrent: false},
		{name: "concurrent_io", delay: 500 * time.Microsecond, workerCount: 8, concurrent: true},
	}

	for _, benchCase := range cases {
		lister := startupBenchmarkWorktreeLister{delay: benchCase.delay}
		b.Run(benchCase.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var summary startupSummary
				if benchCase.concurrent {
					summary = prepareFuzzyStartupConcurrent(cfg, allRepos, sessions, lister, benchCase.workerCount)
				} else {
					summary = prepareFuzzyStartupSequential(cfg, allRepos, sessions, lister)
				}
				if summary.initialWorktrees == 0 {
					b.Fatalf("unexpected empty initial worktree list")
				}
				startupSummarySink = summary
			}
		})
	}
}

func prepareFuzzyStartupSequential(cfg *config.Config, allRepos []github.Repo, sessions []string, lister repoWorktreeLister) startupSummary {
	localRepos := utils.BuildLocalRepoMap(cfg.GetCloneDir(), allRepos)
	openedRepos, openedWorktrees := buildOpenedMapsFromSessions(allRepos, sessions)
	initialWorktrees := loadInitialWorktreesLazy(cfg, allRepos, localRepos, lister)

	return summarizeStartup(localRepos, openedRepos, openedWorktrees, initialWorktrees)
}

func prepareFuzzyStartupConcurrent(cfg *config.Config, allRepos []github.Repo, sessions []string, lister repoWorktreeLister, workers int) startupSummary {
	var (
		localRepos       map[string]bool
		openedRepos      map[string]bool
		openedWorktrees  map[string]map[string]bool
		initialWorktrees int
	)

	var buildWg sync.WaitGroup
	buildWg.Add(2)

	go func() {
		defer buildWg.Done()
		localRepos = utils.BuildLocalRepoMap(cfg.GetCloneDir(), allRepos)
	}()

	go func() {
		defer buildWg.Done()
		openedRepos, openedWorktrees = buildOpenedMapsFromSessions(allRepos, sessions)
	}()

	buildWg.Wait()
	_ = workers
	initialWorktrees = loadInitialWorktreesLazy(cfg, allRepos, localRepos, lister)

	return summarizeStartup(localRepos, openedRepos, openedWorktrees, initialWorktrees)
}

func loadInitialWorktreesLazy(cfg *config.Config, allRepos []github.Repo, localRepos map[string]bool, lister repoWorktreeLister) int {
	for _, repo := range allRepos {
		if !localRepos[repo.FullName] {
			continue
		}

		repoPath := getRepoPath(cfg, repo.FullName, false, repo.DefaultBranch)
		if repoPath == "" {
			return 0
		}

		worktrees, err := lister.ListWorktrees(repoPath)
		if err != nil {
			return 0
		}
		return len(worktrees)
	}
	return 0
}

func summarizeStartup(
	localRepos map[string]bool,
	openedRepos map[string]bool,
	openedWorktrees map[string]map[string]bool,
	initialWorktrees int,
) startupSummary {
	openedWorktreeCount := 0
	for _, byWorktree := range openedWorktrees {
		openedWorktreeCount += len(byWorktree)
	}

	return startupSummary{
		localRepoCount:      len(localRepos),
		openedRepoCount:     len(openedRepos),
		openedWorktreeCount: openedWorktreeCount,
		initialWorktrees:    initialWorktrees,
	}
}
