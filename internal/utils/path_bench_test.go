package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kirksw/ezgit/internal/github"
)

func BenchmarkBuildLocalRepoMap(b *testing.B) {
	cloneDir := b.TempDir()

	repoCounts := []int{100, 500, 1000}
	workers := []int{1, 8}

	for _, repoCount := range repoCounts {
		repos := make([]github.Repo, 0, repoCount)
		for i := 0; i < repoCount; i++ {
			fullName := fmt.Sprintf("acme/repo-%d", i)
			repos = append(repos, github.Repo{FullName: fullName})

			repoPath := filepath.Join(cloneDir, "acme", fmt.Sprintf("repo-%d", i))
			if err := os.MkdirAll(repoPath, 0o755); err != nil {
				b.Fatalf("mkdir %s: %v", repoPath, err)
			}
		}

		for _, workerCount := range workers {
			name := fmt.Sprintf("repos_%d_workers_%d", repoCount, workerCount)
			b.Run(name, func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					local := buildLocalRepoMapWithWorkers(cloneDir, repos, workerCount)
					if len(local) != repoCount {
						b.Fatalf("len(local) = %d, want %d", len(local), repoCount)
					}
				}
			})
		}
	}
}
