package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path     string
	Branch   string
	Bare     bool
	Locked   bool
	Prunable bool
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
	CheckConvertible(path string) error
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
	cmd := exec.Command("git", "worktree", "list", "--porcelain", "-z")
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
	for _, line := range strings.Split(string(output), "\x00") {
		switch {
		case line == "":
			appendCurrent()
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case line == "bare":
			current.Bare = true
		case line == "locked" || strings.HasPrefix(line, "locked "):
			current.Locked = true
		case line == "prunable" || strings.HasPrefix(line, "prunable "):
			current.Prunable = true
		case strings.HasPrefix(line, "branch refs/heads/"):
			current.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch refs/heads/"))
		}
	}
	appendCurrent()
	return worktrees, nil
}

// ListLinkedWorktrees omits the first Git record, which is always the main worktree
// (or bare repository), and excludes registrations Git marks as stale.
func ListLinkedWorktrees(path string) ([]Worktree, error) {
	worktrees, err := New().ListWorktreeDetails(path)
	if err != nil || len(worktrees) == 0 {
		return nil, err
	}
	linked := worktrees[:0]
	for _, wt := range worktrees[1:] {
		if !wt.Bare && !wt.Prunable {
			linked = append(linked, wt)
		}
	}
	return linked, nil
}

func WorktreeName(repoPath, worktreePath string) string {
	if rel, err := filepath.Rel(normalizePathForCompare(repoPath), normalizePathForCompare(worktreePath)); err == nil && rel != "." && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return filepath.Base(worktreePath)
}

// WorktreeSelectors keeps short names when unique and exposes paths otherwise.
func WorktreeSelectors(repoPath string, linked []Worktree) []string {
	selectors := make([]string, len(linked))
	counts := make(map[string]int, len(linked))
	for i, wt := range linked {
		selectors[i] = WorktreeName(repoPath, wt.Path)
		counts[selectors[i]]++
	}
	for i, name := range selectors {
		if counts[name] > 1 {
			selectors[i] = linked[i].Path
		}
	}
	return selectors
}

func (g *gitManager) ListWorktrees(path string) ([]string, error) {
	linked, err := ListLinkedWorktrees(path)
	if err != nil || len(linked) == 0 {
		return nil, err
	}
	return WorktreeSelectors(path, linked), nil
}
