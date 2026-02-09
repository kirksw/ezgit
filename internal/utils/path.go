package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	result := make(map[string]bool)
	if cloneDir == "" {
		return result
	}
	for _, repo := range repos {
		parts := strings.Split(repo.FullName, "/")
		if len(parts) != 2 {
			continue
		}
		dir := filepath.Join(cloneDir, parts[0], parts[1])
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			result[repo.FullName] = true
		}
	}
	return result
}

func ValidatePath(path string) error {
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid path: %s", path)
	}
	return nil
}
