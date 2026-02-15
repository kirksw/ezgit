package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
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

func makeCommit(t *testing.T, repoDir, fileName, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, fileName), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", fileName, err)
	}
	runGit(t, repoDir, "add", fileName)
	runGit(t, repoDir, "commit", "-m", message)
}

func TestConfigureBareRemoteSetsUpRemoteTracking(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a "remote" bare repo with two branches.
	originDir := filepath.Join(tmpDir, "origin.git")
	seedDir := filepath.Join(tmpDir, "seed")

	runGit(t, "", "init", "--bare", originDir)
	runGit(t, "", "clone", originDir, seedDir)
	runGit(t, seedDir, "config", "user.email", "test@example.com")
	runGit(t, seedDir, "config", "user.name", "test")
	makeCommit(t, seedDir, "README.md", "hello\n", "initial commit")
	runGit(t, seedDir, "push", "origin", "HEAD:main")

	runGit(t, seedDir, "checkout", "-b", "feature-a")
	makeCommit(t, seedDir, "a.txt", "a\n", "feature-a commit")
	runGit(t, seedDir, "push", "origin", "feature-a")

	// Bare-clone (mimics what ezgit does).
	bareDir := filepath.Join(tmpDir, "repo", ".git")
	runGit(t, "", "clone", "--bare", originDir, bareDir)

	// Verify broken state BEFORE fix: no fetch refspec, branches are local.
	cfgCmd := exec.Command("git", "config", "--get", "remote.origin.fetch")
	cfgCmd.Dir = bareDir
	if err := cfgCmd.Run(); err == nil {
		t.Fatal("expected no remote.origin.fetch before ConfigureBareRemote")
	}

	// Apply fix.
	gitMgr := New()
	if err := gitMgr.ConfigureBareRemote(bareDir, "main"); err != nil {
		t.Fatalf("ConfigureBareRemote() error = %v", err)
	}

	// Verify fetch refspec is set.
	refspec := strings.TrimSpace(runGit(t, bareDir, "config", "--get", "remote.origin.fetch"))
	if refspec != "+refs/heads/*:refs/remotes/origin/*" {
		t.Fatalf("remote.origin.fetch = %q, want +refs/heads/*:refs/remotes/origin/*", refspec)
	}

	// Verify remote-tracking branches exist.
	remoteBranches := strings.TrimSpace(runGit(t, bareDir, "branch", "-r", "--format=%(refname:short)"))
	if !strings.Contains(remoteBranches, "origin/main") {
		t.Fatalf("expected origin/main in remote branches, got: %s", remoteBranches)
	}
	if !strings.Contains(remoteBranches, "origin/feature-a") {
		t.Fatalf("expected origin/feature-a in remote branches, got: %s", remoteBranches)
	}

	// Verify stale local branches were cleaned up (only main should remain).
	localBranches := strings.TrimSpace(runGit(t, bareDir, "branch", "--format=%(refname:short)"))
	for _, b := range strings.Split(localBranches, "\n") {
		b = strings.TrimSpace(b)
		if b != "" && b != "main" {
			t.Fatalf("expected only main as local branch, but found %q in: %s", b, localBranches)
		}
	}
}

func TestConvertToBareProducesBareRepository(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	runGit(t, "", "init", repoDir)
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "test")
	makeCommit(t, repoDir, "README.md", "hello\n", "initial commit")

	gitMgr := New()
	if err := gitMgr.ConvertToBare(repoDir); err != nil {
		t.Fatalf("ConvertToBare() error = %v", err)
	}

	gitDir := filepath.Join(repoDir, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		t.Fatalf("expected .git directory to exist after conversion, got err=%v", err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("expected working tree files to be removed, got err=%v", err)
	}

	out := strings.TrimSpace(runGit(t, "", "--git-dir", gitDir, "rev-parse", "--is-bare-repository"))
	if out != "true" {
		t.Fatalf("is-bare-repository = %q, want true", out)
	}
}

func TestListBranchesSkipsSymbolicOriginRef(t *testing.T) {
	tmpDir := t.TempDir()
	originDir := filepath.Join(tmpDir, "origin.git")
	seedDir := filepath.Join(tmpDir, "seed")
	workDir := filepath.Join(tmpDir, "work")

	runGit(t, "", "init", "--bare", originDir)
	runGit(t, "", "clone", originDir, seedDir)
	runGit(t, seedDir, "config", "user.email", "test@example.com")
	runGit(t, seedDir, "config", "user.name", "test")
	makeCommit(t, seedDir, "README.md", "hello\n", "main commit")
	runGit(t, seedDir, "push", "origin", "HEAD:main")

	runGit(t, seedDir, "checkout", "-b", "develop")
	makeCommit(t, seedDir, "develop.txt", "dev\n", "develop commit")
	runGit(t, seedDir, "push", "origin", "develop")

	runGit(t, "", "clone", originDir, workDir)

	gitMgr := New()
	branches, err := gitMgr.ListBranches(workDir)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}

	foundMain := false
	foundDevelop := false
	for _, b := range branches {
		if b == "main" {
			foundMain = true
		}
		if b == "develop" {
			foundDevelop = true
		}
		if b == "origin" || b == "HEAD" {
			t.Fatalf("ListBranches() returned symbolic ref %q", b)
		}
	}

	if !foundMain || !foundDevelop {
		t.Fatalf("ListBranches() = %v, expected to contain main and develop", branches)
	}
}

func TestCreateWorktreeRelativePathUsesRepoRoot(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	runGit(t, "", "init", repoDir)
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "test")
	makeCommit(t, repoDir, "README.md", "hello\n", "initial commit")

	branchName := strings.TrimSpace(runGit(t, repoDir, "branch", "--show-current"))
	if branchName == "" {
		t.Fatal("expected current branch name")
	}

	gitMgr := New()
	if err := gitMgr.ConvertToBare(repoDir); err != nil {
		t.Fatalf("ConvertToBare() error = %v", err)
	}

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

	relativeWorktreePath := filepath.Join("repo", branchName)
	if err := gitMgr.CreateWorktree(filepath.Join(repoDir, ".git"), relativeWorktreePath, branchName); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	expectedWorktreePath := filepath.Join(repoDir, branchName)
	if info, err := os.Stat(expectedWorktreePath); err != nil || !info.IsDir() {
		t.Fatalf("expected worktree at %s, err=%v", expectedWorktreePath, err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, ".git", "repo")); !os.IsNotExist(err) {
		t.Fatalf("expected no nested worktree directory under .git/repo, got err=%v", err)
	}
}

func TestListWorktreesSkipsBareMetadataPath(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	runGit(t, "", "init", repoDir)
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "test")
	makeCommit(t, repoDir, "README.md", "hello\n", "initial commit")

	branchName := strings.TrimSpace(runGit(t, repoDir, "branch", "--show-current"))
	if branchName == "" {
		t.Fatal("expected current branch name")
	}

	gitMgr := New()
	if err := gitMgr.ConvertToBare(repoDir); err != nil {
		t.Fatalf("ConvertToBare() error = %v", err)
	}
	if err := gitMgr.CreateWorktree(filepath.Join(repoDir, ".git"), filepath.Join(repoDir, branchName), branchName); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	worktrees, err := gitMgr.ListWorktrees(repoDir)
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}

	foundBranch := false
	for _, wt := range worktrees {
		if wt == ".git" {
			t.Fatalf("ListWorktrees() returned metadata path: %v", worktrees)
		}
		if wt == branchName {
			foundBranch = true
		}
	}
	if !foundBranch {
		t.Fatalf("ListWorktrees() = %v, expected %q", worktrees, branchName)
	}
}

func TestCreateDetachedWorktreeFromDefaultBranch(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	runGit(t, "", "init", repoDir)
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "test")
	makeCommit(t, repoDir, "README.md", "hello\n", "initial commit")

	defaultBranch := strings.TrimSpace(runGit(t, repoDir, "branch", "--show-current"))
	if defaultBranch == "" {
		t.Fatal("expected current branch name")
	}

	gitMgr := New()
	if err := gitMgr.ConvertToBare(repoDir); err != nil {
		t.Fatalf("ConvertToBare() error = %v", err)
	}

	defaultWorktreePath := filepath.Join(repoDir, defaultBranch)
	if err := gitMgr.CreateWorktree(filepath.Join(repoDir, ".git"), defaultWorktreePath, defaultBranch); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	reviewPath := filepath.Join(repoDir, "review")
	if err := gitMgr.CreateDetachedWorktree(filepath.Join(repoDir, ".git"), reviewPath, defaultBranch); err != nil {
		t.Fatalf("CreateDetachedWorktree() error = %v", err)
	}

	head := strings.TrimSpace(runGit(t, reviewPath, "rev-parse", "--abbrev-ref", "HEAD"))
	if head != "HEAD" {
		t.Fatalf("expected detached HEAD in review worktree, got %q", head)
	}
}

func TestCreateFeatureWorktreeFromBaseBranch(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	runGit(t, "", "init", repoDir)
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "test")
	makeCommit(t, repoDir, "README.md", "hello\n", "initial commit")

	defaultBranch := strings.TrimSpace(runGit(t, repoDir, "branch", "--show-current"))
	if defaultBranch == "" {
		t.Fatal("expected current branch name")
	}

	gitMgr := New()
	if err := gitMgr.ConvertToBare(repoDir); err != nil {
		t.Fatalf("ConvertToBare() error = %v", err)
	}

	defaultWorktreePath := filepath.Join(repoDir, defaultBranch)
	if err := gitMgr.CreateWorktree(filepath.Join(repoDir, ".git"), defaultWorktreePath, defaultBranch); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	featureBranch := "feature/test-worktree"
	featurePath := filepath.Join(repoDir, featureBranch)
	if err := gitMgr.CreateFeatureWorktree(filepath.Join(repoDir, ".git"), featurePath, featureBranch, defaultBranch); err != nil {
		t.Fatalf("CreateFeatureWorktree() error = %v", err)
	}

	head := strings.TrimSpace(runGit(t, featurePath, "rev-parse", "--abbrev-ref", "HEAD"))
	if head != featureBranch {
		t.Fatalf("feature worktree HEAD = %q, want %q", head, featureBranch)
	}
}

func TestCreateDetachedWorktreeIsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	runGit(t, "", "init", repoDir)
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "test")
	makeCommit(t, repoDir, "README.md", "hello\n", "initial commit")

	defaultBranch := strings.TrimSpace(runGit(t, repoDir, "branch", "--show-current"))
	if defaultBranch == "" {
		t.Fatal("expected current branch name")
	}

	gitMgr := New()
	if err := gitMgr.ConvertToBare(repoDir); err != nil {
		t.Fatalf("ConvertToBare() error = %v", err)
	}

	defaultWorktreePath := filepath.Join(repoDir, defaultBranch)
	if err := gitMgr.CreateWorktree(filepath.Join(repoDir, ".git"), defaultWorktreePath, defaultBranch); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	reviewPath := filepath.Join(repoDir, "review")
	if err := gitMgr.CreateDetachedWorktree(filepath.Join(repoDir, ".git"), reviewPath, defaultBranch); err != nil {
		t.Fatalf("CreateDetachedWorktree(first) error = %v", err)
	}
	if err := gitMgr.CreateDetachedWorktree(filepath.Join(repoDir, ".git"), reviewPath, defaultBranch); err != nil {
		t.Fatalf("CreateDetachedWorktree(second) error = %v", err)
	}

	head := strings.TrimSpace(runGit(t, reviewPath, "rev-parse", "--abbrev-ref", "HEAD"))
	if head != "HEAD" {
		t.Fatalf("expected detached HEAD in review worktree, got %q", head)
	}
}

func TestCreateDetachedWorktreeFailsWhenPathExistsButNotRegistered(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	runGit(t, "", "init", repoDir)
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "test")
	makeCommit(t, repoDir, "README.md", "hello\n", "initial commit")

	defaultBranch := strings.TrimSpace(runGit(t, repoDir, "branch", "--show-current"))
	if defaultBranch == "" {
		t.Fatal("expected current branch name")
	}

	gitMgr := New()
	if err := gitMgr.ConvertToBare(repoDir); err != nil {
		t.Fatalf("ConvertToBare() error = %v", err)
	}

	defaultWorktreePath := filepath.Join(repoDir, defaultBranch)
	if err := gitMgr.CreateWorktree(filepath.Join(repoDir, ".git"), defaultWorktreePath, defaultBranch); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	reviewPath := filepath.Join(repoDir, "review")
	if err := os.MkdirAll(reviewPath, 0755); err != nil {
		t.Fatalf("failed to create review path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewPath, "README.md"), []byte("not a worktree"), 0644); err != nil {
		t.Fatalf("failed to write marker file: %v", err)
	}

	err := gitMgr.CreateDetachedWorktree(filepath.Join(repoDir, ".git"), reviewPath, defaultBranch)
	if err == nil {
		t.Fatal("expected error when worktree path exists but is not registered")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}
