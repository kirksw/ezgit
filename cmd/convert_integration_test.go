//go:build integration

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const integrationRepoURL = "https://github.com/kirksw/kirksw.git"

func runGitIntegration(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func TestConvertGitHubCloneIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "kirksw")

	runGitIntegration(t, "", "clone", "--depth", "1", integrationRepoURL, repoPath)

	prevWorktrees := worktrees
	prevAllWorktrees := allWorktrees
	prevNoWorktrees := noWorktrees
	t.Cleanup(func() {
		worktrees = prevWorktrees
		allWorktrees = prevAllWorktrees
		noWorktrees = prevNoWorktrees
	})

	worktrees = nil
	allWorktrees = true
	noWorktrees = false

	if err := runConvert(nil, []string{repoPath}); err != nil {
		t.Fatalf("runConvert() error = %v", err)
	}

	gitDir := filepath.Join(repoPath, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		t.Fatalf("expected .git directory to exist after conversion, got err=%v", err)
	}

	out := strings.TrimSpace(runGitIntegration(t, "", "--git-dir", gitDir, "rev-parse", "--is-bare-repository"))
	if out != "true" {
		t.Fatalf("is-bare-repository = %q, want true", out)
	}

	entries, err := os.ReadDir(repoPath)
	if err != nil {
		t.Fatalf("ReadDir(%s) failed: %v", repoPath, err)
	}

	branchDirCount := 0
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".git" {
			branchDirCount++
		}
	}
	if branchDirCount == 0 {
		t.Fatalf("expected at least one branch worktree directory under %s", repoPath)
	}
}
