package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGitCmd(t *testing.T, dir string, args ...string) string {
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

func TestConvertAllWorktreesWithRelativePathCreatesBranchDirsAtRepoRoot(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	runGitCmd(t, "", "init", repoDir)
	runGitCmd(t, repoDir, "config", "user.email", "test@example.com")
	runGitCmd(t, repoDir, "config", "user.name", "test")

	readmePath := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("failed to write README.md: %v", err)
	}
	runGitCmd(t, repoDir, "add", "README.md")
	runGitCmd(t, repoDir, "commit", "-m", "initial")

	oldWorktrees := worktrees
	oldAllWorktrees := allWorktrees
	oldNoWorktrees := noWorktrees
	t.Cleanup(func() {
		worktrees = oldWorktrees
		allWorktrees = oldAllWorktrees
		noWorktrees = oldNoWorktrees
	})

	worktrees = nil
	allWorktrees = true
	noWorktrees = false

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%s) failed: %v", tmpDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWd)
	})

	if err := runConvert(nil, []string{"./repo"}); err != nil {
		t.Fatalf("runConvert() error = %v", err)
	}

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		t.Fatalf("ReadDir(%s) failed: %v", repoDir, err)
	}

	branchDirCount := 0
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".git" {
			branchDirCount++
		}
	}
	if branchDirCount == 0 {
		t.Fatalf("expected at least one branch worktree directory under %s", repoDir)
	}

	if _, err := os.Stat(filepath.Join(repoDir, ".git", "repo")); !os.IsNotExist(err) {
		t.Fatalf("expected no nested worktree directory under .git/repo, got err=%v", err)
	}
}

func TestResolveConvertDefaultBranchPrefersMain(t *testing.T) {
	got := resolveConvertDefaultBranch("", []string{"develop", "main", "release"})
	if got != "main" {
		t.Fatalf("got %q, want %q", got, "main")
	}
}

func TestResolveConvertDefaultBranchFallsBackToMaster(t *testing.T) {
	got := resolveConvertDefaultBranch("", []string{"develop", "master", "release"})
	if got != "master" {
		t.Fatalf("got %q, want %q", got, "master")
	}
}

func TestResolveConvertDefaultBranchUsesExplicitWhenPresent(t *testing.T) {
	got := resolveConvertDefaultBranch("trunk", []string{"trunk", "develop"})
	if got != "trunk" {
		t.Fatalf("got %q, want %q", got, "trunk")
	}
}
