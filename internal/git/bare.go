package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const maxConvertBlockersShown = 10

// ConfigureBareRemote fixes a bare clone so it has proper remote tracking.
// git clone --bare does not set a fetch refspec and converts all remote
// branches into local branches.  This method:
//  1. Sets remote.origin.fetch so future fetches work.
//  2. Runs git fetch to populate remote-tracking refs.
//  3. Removes the stale local branches that --bare created (keeping only
//     the default branch).
func (g *gitManager) ConfigureBareRemote(barePath, defaultBranch string) error {
	// 1. Set the fetch refspec that --bare omits.
	if err := runGitCommand(barePath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return fmt.Errorf("failed to set fetch refspec: %w", err)
	}

	// 2. Fetch to create proper remote-tracking refs.
	if err := runGitCommand(barePath, "fetch", "origin"); err != nil {
		return fmt.Errorf("failed to fetch from origin: %w", err)
	}

	// 3. Delete local branches that --bare created, except the default branch.
	//    These are now redundant because remote-tracking refs exist.
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = barePath
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list local branches: %w", err)
	}
	for _, branch := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		branch = strings.TrimSpace(branch)
		if branch == "" || branch == defaultBranch {
			continue
		}
		if delErr := runGitCommand(barePath, "branch", "-D", branch); delErr != nil {
			// Non-fatal: log and continue.
			continue
		}
	}

	return nil
}

// CheckConvertible returns an error unless path is a regular repository whose
// working tree can be deleted without losing anything: ConvertToBare removes
// every file outside .git, so anything git could not restore must block it.
func (g *gitManager) CheckConvertible(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	gitDir := filepath.Join(path, ".git")
	info, err := os.Lstat(gitDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", path)
	}
	if err != nil {
		return fmt.Errorf("failed to inspect %s: %w", gitDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("cannot convert a linked worktree or submodule: %s", path)
	}

	isBare, err := gitOutput(path, "--git-dir", gitDir, "rev-parse", "--is-bare-repository")
	if err != nil {
		return fmt.Errorf("not a git repository: %s: %w", path, err)
	}
	if strings.TrimSpace(isBare) == "true" {
		return fmt.Errorf("repository already uses worktree layout: %s", path)
	}

	worktreeList, err := gitOutput(path, "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}
	if strings.Count("\n"+worktreeList, "\nworktree ") > 1 {
		return fmt.Errorf("repository has linked worktrees; remove them before converting: %s", path)
	}

	status, err := gitOutput(path, "status", "--porcelain", "--untracked-files=normal", "--ignored=traditional", "--ignore-submodules=none")
	if err != nil {
		return fmt.Errorf("failed to inspect working tree: %w", err)
	}
	blockers := nonEmptyLines(status)

	// Files flagged assume-unchanged or skip-worktree are hidden from status.
	tracked, err := gitOutput(path, "ls-files", "-v")
	if err != nil {
		return fmt.Errorf("failed to inspect tracked files: %w", err)
	}
	for _, line := range nonEmptyLines(tracked) {
		if tag := line[0]; tag == 'S' || (tag >= 'a' && tag <= 'z') {
			blockers = append(blockers, line)
		}
	}

	if len(blockers) > 0 {
		return fmt.Errorf(
			"working tree has changes that converting would delete; commit, move, or remove them first:\n%s",
			summarizeLines(blockers, maxConvertBlockersShown),
		)
	}
	return nil
}

// ConvertToBare keeps the existing .git directory, so remotes, branches, stashes,
// config, and hooks all survive the conversion.
func (g *gitManager) ConvertToBare(path string) error {
	if err := g.CheckConvertible(path); err != nil {
		return err
	}

	if _, err := gitOutput(path, "config", "core.bare", "true"); err != nil {
		return fmt.Errorf("failed to mark repository as bare: %w", err)
	}

	items, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read original repository: %w", err)
	}
	for _, item := range items {
		if item.Name() == ".git" {
			continue
		}
		itemPath := filepath.Join(path, item.Name())
		if err := os.RemoveAll(itemPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", itemPath, err)
		}
	}

	indexPath := filepath.Join(path, ".git", "index")
	if err := os.Remove(indexPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s: %w", indexPath, err)
	}

	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return string(output), nil
}

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func summarizeLines(lines []string, limit int) string {
	shown := lines
	if len(shown) > limit {
		shown = shown[:limit]
	}
	summary := "  " + strings.Join(shown, "\n  ")
	if hidden := len(lines) - len(shown); hidden > 0 {
		summary += fmt.Sprintf("\n  ... and %d more", hidden)
	}
	return summary
}

func (g *gitManager) CreateWorktree(barePath, worktreePath, branch string) error {
	return g.runWorktreeAdd(barePath, worktreePath, nil, []string{branch})
}

func (g *gitManager) CreateDetachedWorktree(barePath, worktreePath, startPoint string) error {
	return g.runWorktreeAdd(barePath, worktreePath, []string{"--detach"}, []string{startPoint})
}

func (g *gitManager) CreateFeatureWorktree(barePath, worktreePath, featureBranch, baseBranch string) error {
	return g.runWorktreeAdd(barePath, worktreePath, []string{"-b", featureBranch}, []string{baseBranch})
}

func (g *gitManager) runWorktreeAdd(barePath, worktreePath string, prePathArgs []string, postPathArgs []string) error {
	absWorktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to resolve worktree path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absWorktreePath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Idempotency guard: if this exact worktree path is already registered,
	// treat the operation as successful.
	alreadyRegistered, err := g.isWorktreeRegistered(barePath, absWorktreePath)
	if err != nil {
		return fmt.Errorf("failed to inspect existing worktrees: %w", err)
	}
	if alreadyRegistered {
		return nil
	}

	gitArgs := []string{"worktree", "add"}
	gitArgs = append(gitArgs, prePathArgs...)
	gitArgs = append(gitArgs, absWorktreePath)
	gitArgs = append(gitArgs, postPathArgs...)
	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = barePath
	if output, err := cmd.CombinedOutput(); err != nil {
		// If another code path created the same worktree just before this call,
		// tolerate the "already exists" error only when the worktree is now registered.
		if strings.Contains(string(output), "already exists") {
			registeredNow, checkErr := g.isWorktreeRegistered(barePath, absWorktreePath)
			if checkErr == nil && registeredNow {
				return nil
			}
		}
		return fmt.Errorf("failed to create worktree: %w\n%s", err, string(output))
	}

	return nil
}

func (g *gitManager) isWorktreeRegistered(barePath, worktreePath string) (bool, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = barePath
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	target := normalizePathForCompare(worktreePath)
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		if path == "" {
			continue
		}
		if normalizePathForCompare(path) == target {
			return true, nil
		}
	}

	return false, nil
}

func normalizePathForCompare(path string) string {
	normalized := filepath.Clean(path)
	if abs, err := filepath.Abs(normalized); err == nil {
		normalized = abs
	}
	if eval, err := filepath.EvalSymlinks(normalized); err == nil {
		normalized = eval
	}
	return normalized
}

func (g *gitManager) ListBranches(path string) ([]string, error) {
	remoteCmd := exec.Command("git", "branch", "-r", "--format=%(refname:short)")
	remoteCmd.Dir = path
	remoteOutput, err := remoteCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	localCmd := exec.Command("git", "branch", "--format=%(refname:short)")
	localCmd.Dir = path
	localOutput, err := localCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list local branches: %w", err)
	}

	seen := make(map[string]struct{})
	addBranch := func(branch string) {
		branch = strings.TrimSpace(branch)
		branch = strings.TrimPrefix(branch, "origin/")
		if branch == "" || branch == "HEAD" || branch == "origin" || strings.Contains(branch, "->") {
			return
		}
		seen[branch] = struct{}{}
	}

	lines := strings.Split(strings.TrimSpace(string(remoteOutput)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			addBranch(line)
		}
	}

	lines = strings.Split(strings.TrimSpace(string(localOutput)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			addBranch(line)
		}
	}

	branches := make([]string, 0, len(seen))
	for branch := range seen {
		branches = append(branches, branch)
	}
	sort.Strings(branches)

	return branches, nil
}
