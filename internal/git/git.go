package git

import (
	"os/exec"
	"strings"
)

type CloneOptions struct {
	Bare       bool
	Branch     string
	Depth      int
	Quiet      bool
	SSHKeyPath string
}

type GitManager interface {
	Clone(url, path string, opts CloneOptions) error
	ConvertToBare(path string) error
	CreateWorktree(barePath, worktreePath, branch string) error
	ListBranches(path string) ([]string, error)
	ValidateSSHKey(path string) error
	HasWorktrees(path string) (bool, error)
	ListWorktrees(path string) ([]string, error)
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
	return len(worktrees) > 1, nil
}

func (g *gitManager) ListWorktrees(path string) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var worktrees []string
	var currentPath string

	for _, line := range lines {
		if line == "" {
			if currentPath != "" && currentPath != path {
				name := strings.TrimPrefix(currentPath, path+"/")
				if name == "" {
					name = "main"
				}
				worktrees = append(worktrees, name)
				currentPath = ""
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				currentPath = parts[1]
			}
		}
	}

	if currentPath != "" && currentPath != path {
		name := strings.TrimPrefix(currentPath, path+"/")
		if name == "" {
			name = "main"
		}
		worktrees = append(worktrees, name)
	}

	return worktrees, nil
}
