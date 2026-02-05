package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (g *gitManager) ConvertToBare(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", path)
	}

	tempBareDir := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".bare")

	cloneArgs := []string{"clone", "--bare", path, tempBareDir}
	cmd := exec.Command("git", cloneArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create bare clone: %w\n%s", err, string(output))
	}

	items, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, item := range items {
		if item.Name() == ".git" {
			continue
		}
		itemPath := filepath.Join(path, item.Name())
		if err := os.RemoveAll(itemPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", item.Name(), err)
		}
	}

	bareItems, err := os.ReadDir(tempBareDir)
	if err != nil {
		return fmt.Errorf("failed to read bare directory: %w", err)
	}

	for _, item := range bareItems {
		src := filepath.Join(tempBareDir, item.Name())
		dst := filepath.Join(path, item.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("failed to move %s: %w", item.Name(), err)
		}
	}

	if err := os.RemoveAll(tempBareDir); err != nil {
		return fmt.Errorf("failed to remove temp dir: %w", err)
	}

	configPath := filepath.Join(path, "config")
	if err := os.WriteFile(configPath, []byte("core.bare = true\n"), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (g *gitManager) CreateWorktree(barePath, worktreePath, branch string) error {
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	args := []string{"worktree", "add", worktreePath, branch}
	cmd := exec.Command("git", args...)
	cmd.Dir = barePath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create worktree: %w\n%s", err, string(output))
	}

	return nil
}

func (g *gitManager) ListBranches(path string) ([]string, error) {
	cmd := exec.Command("git", "branch", "-r", "--format=%(refname:short)")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	branches := []string{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}

	return branches, nil
}
