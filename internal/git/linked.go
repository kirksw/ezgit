package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ResolveCommit(repoPath, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cannot resolve %q to a commit: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func RemoteDefaultRef(repoPath string) string {
	cmd := exec.Command("git", "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	cmd.Dir = repoPath
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// AddLinkedWorktree creates a new branch. Unlike the legacy layout helpers,
// it must not accept an existing worktree, which could belong to another writer.
func AddLinkedWorktree(repoPath, path, branch, commit string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create worktree parent: %w", err)
	}
	cmd := exec.Command("git", "worktree", "add", "-b", branch, "--", path, commit)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create worktree: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// WorktreeHasChanges also checks ignored files, which Git worktree remove can discard.
func WorktreeHasChanges(path string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain=v1", "-z", "--untracked-files=normal", "--ignored")
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check worktree %s: %w\n%s", path, err, strings.TrimSpace(string(out)))
	}
	return len(out) != 0, nil
}
