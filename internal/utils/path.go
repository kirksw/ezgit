package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/kirksw/ezgit/internal/github"
)

func ParseWorktreePath(basePath, branch string) string {
	basePath = strings.TrimSpace(basePath)
	branch = strings.TrimSpace(branch)

	return filepath.Join(basePath, branch)
}

// BuildLocalRepoMap returns a map of FullName->true for repos whose
// directories already exist under cloneDir/{owner}/{repo}.
func BuildLocalRepoMap(cloneDir string, repos []github.Repo) map[string]bool {
	return buildLocalRepoMapWithWorkers(cloneDir, repos, defaultLocalRepoLookupWorkers())
}

func buildLocalRepoMapWithWorkers(cloneDir string, repos []github.Repo, workers int) map[string]bool {
	result := make(map[string]bool)
	if cloneDir == "" {
		return result
	}
	if len(repos) == 0 {
		return result
	}

	if workers < 1 {
		workers = 1
	}
	if workers > len(repos) {
		workers = len(repos)
	}

	jobs := make(chan github.Repo)
	var wg sync.WaitGroup
	var resultMu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repo := range jobs {
				owner, name, ok := strings.Cut(repo.FullName, "/")
				if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
					continue
				}

				dir := filepath.Join(cloneDir, owner, name)
				if info, err := os.Stat(dir); err == nil && info.IsDir() {
					resultMu.Lock()
					result[repo.FullName] = true
					resultMu.Unlock()
				}
			}
		}()
	}

	for _, repo := range repos {
		jobs <- repo
	}
	close(jobs)
	wg.Wait()

	return result
}

func defaultLocalRepoLookupWorkers() int {
	workers := runtime.GOMAXPROCS(0)
	if workers < 4 {
		return 4
	}
	if workers > 16 {
		return 16
	}
	return workers
}

func ValidatePath(path string) error {
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid path: %s", path)
	}
	return nil
}
