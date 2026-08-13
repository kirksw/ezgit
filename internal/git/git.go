package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path   string
	Branch string
}

type CloneOptions struct {
	Bare       bool
	Branch     string
	Depth      int
	Quiet      bool
	SSHKeyPath string
}

type GitManager interface {
	Clone(url, path string, opts CloneOptions) error
	ConfigureBareRemote(barePath, defaultBranch string) error
	ConvertToBare(path string) error
	CreateWorktree(barePath, worktreePath, branch string) error
	CreateDetachedWorktree(barePath, worktreePath, startPoint string) error
	CreateFeatureWorktree(barePath, worktreePath, featureBranch, baseBranch string) error
	ListBranches(path string) ([]string, error)
	ValidateSSHKey(path string) error
	HasWorktrees(path string) (bool, error)
	ListWorktrees(path string) ([]string, error)
	ListWorktreeDetails(path string) ([]Worktree, error)
	RemoveWorktree(path, worktreePath string, force bool) error
}

type gitManager struct{}

func New() GitManager {
	return &gitManager{}
}

func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Run()
}

func (g *gitManager) HasWorktrees(path string) (bool, error) {
	worktrees, err := g.ListWorktrees(path)
	if err != nil {
		return false, err
	}
	return len(worktrees) > 0, nil
}

func (g *gitManager) RemoveWorktree(path, worktreePath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (g *gitManager) ListWorktreeDetails(path string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var worktrees []Worktree
	var current Worktree
	appendCurrent := func() {
		if current.Path != "" {
			worktrees = append(worktrees, current)
		}
		current = Worktree{}
	}
	for _, line := range strings.Split(string(output), "\n") {
		switch {
		case line == "":
			appendCurrent()
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.HasPrefix(line, "branch refs/heads/"):
			current.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch refs/heads/"))
		}
	}
	appendCurrent()
	return worktrees, nil
}

func (g *gitManager) ListWorktrees(path string) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	repoPath := filepath.Clean(path)
	if abs, err := filepath.Abs(repoPath); err == nil {
		repoPath = abs
	}
	metadataPath := filepath.Join(repoPath, ".git")

	lines := strings.Split(string(output), "\n")
	var worktrees []string
	seen := make(map[string]struct{})
	var currentPath string

	appendIfWorktree := func(p string) {
		if p == "" {
			return
		}

		cleanPath := filepath.Clean(strings.TrimSpace(p))
		if abs, err := filepath.Abs(cleanPath); err == nil {
			cleanPath = abs
		}

		if cleanPath == repoPath || cleanPath == metadataPath {
			return
		}

		name := filepath.Base(cleanPath)
		if strings.HasPrefix(cleanPath, repoPath+string(filepath.Separator)) {
			name = strings.TrimPrefix(cleanPath, repoPath+string(filepath.Separator))
		}
		name = strings.TrimSpace(name)
		if name == "" || name == ".git" {
			return
		}

		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		worktrees = append(worktrees, name)
	}

	for _, line := range lines {
		if line == "" {
			appendIfWorktree(currentPath)
			currentPath = ""
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				currentPath = parts[1]
			}
		}
	}

	appendIfWorktree(currentPath)

	return worktrees, nil
}
