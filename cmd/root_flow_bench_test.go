package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
)

type benchmarkWorktreeLister struct {
	delay time.Duration
}

func (l benchmarkWorktreeLister) ListWorktrees(path string) ([]string, error) {
	if l.delay > 0 {
		time.Sleep(l.delay)
	}
	return []string{"main", "review"}, nil
}

func BenchmarkBuildLocalRepoWorktreeMap(b *testing.B) {
	cloneDir := b.TempDir()
	cfg := &config.Config{Git: config.GitConfig{CloneDir: cloneDir}}

	repoCounts := []int{50, 250}
	workers := []int{1, 8}
	delays := []struct {
		name  string
		delay time.Duration
	}{
		{name: "cpu_only", delay: 0},
		{name: "simulated_io", delay: 500 * time.Microsecond},
	}

	for _, delayCase := range delays {
		lister := benchmarkWorktreeLister{delay: delayCase.delay}
		for _, repoCount := range repoCounts {
			allRepos := make([]github.Repo, 0, repoCount)
			localRepos := make(map[string]bool, repoCount)
			for i := 0; i < repoCount; i++ {
				fullName := fmt.Sprintf("acme/repo-%d", i)
				allRepos = append(allRepos, github.Repo{FullName: fullName})
				localRepos[fullName] = true
			}

			for _, workerCount := range workers {
				name := fmt.Sprintf("%s/repos_%d_workers_%d", delayCase.name, repoCount, workerCount)
				b.Run(name, func(b *testing.B) {
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_ = buildLocalRepoWorktreeMapWithLister(cfg, allRepos, localRepos, lister, workerCount)
					}
				})
			}
		}
	}
}
