package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kirksw/ezgit/internal/config"
)

func TestResolveOpenCommandTemplateDefaults(t *testing.T) {
	cfg := &config.Config{}
	got := resolveOpenCommandTemplate(cfg)
	if got != defaultOpenCommandTemplate {
		t.Fatalf("resolveOpenCommandTemplate()=%q, want %q", got, defaultOpenCommandTemplate)
	}
}

func TestResolveOpenCommandTemplateUsesConfig(t *testing.T) {
	cfg := &config.Config{
		Git: config.GitConfig{
			OpenCommand: `tmux new-window -c "$absPath"`,
		},
	}
	got := resolveOpenCommandTemplate(cfg)
	if got != `tmux new-window -c "$absPath"` {
		t.Fatalf("resolveOpenCommandTemplate()=%q, want configured command", got)
	}
}

func TestBuildOpenCommandContextWithWorktree(t *testing.T) {
	cfg := &config.Config{
		Git: config.GitConfig{
			CloneDir: "/tmp/repos",
		},
	}

	ctx, err := buildOpenCommandContext(cfg, "acme/widgets", "review")
	if err != nil {
		t.Fatalf("buildOpenCommandContext() error = %v", err)
	}

	if ctx.Org != "acme" {
		t.Fatalf("Org=%q, want %q", ctx.Org, "acme")
	}
	if ctx.Repo != "widgets" {
		t.Fatalf("Repo=%q, want %q", ctx.Repo, "widgets")
	}
	if ctx.Worktree != "review" {
		t.Fatalf("Worktree=%q, want %q", ctx.Worktree, "review")
	}
	if ctx.OrgRepo != "acme/widgets" {
		t.Fatalf("OrgRepo=%q, want %q", ctx.OrgRepo, "acme/widgets")
	}
	if ctx.RepoPath != "acme/widgets/review" {
		t.Fatalf("RepoPath=%q, want %q", ctx.RepoPath, "acme/widgets/review")
	}
	wantAbsPath := filepath.Join("/tmp/repos", "acme", "widgets", "review")
	if ctx.AbsPath != wantAbsPath {
		t.Fatalf("AbsPath=%q, want %q", ctx.AbsPath, wantAbsPath)
	}
}

func TestBuildOpenCommandContextWithoutWorktree(t *testing.T) {
	cfg := &config.Config{
		Git: config.GitConfig{
			CloneDir: "/tmp/repos",
		},
	}

	ctx, err := buildOpenCommandContext(cfg, "acme/widgets", "")
	if err != nil {
		t.Fatalf("buildOpenCommandContext() error = %v", err)
	}

	if ctx.Worktree != "" {
		t.Fatalf("Worktree=%q, want empty", ctx.Worktree)
	}
	if ctx.RepoPath != "acme/widgets" {
		t.Fatalf("RepoPath=%q, want %q", ctx.RepoPath, "acme/widgets")
	}
	wantAbsPath := filepath.Join("/tmp/repos", "acme", "widgets")
	if ctx.AbsPath != wantAbsPath {
		t.Fatalf("AbsPath=%q, want %q", ctx.AbsPath, wantAbsPath)
	}
}

func TestBuildOpenCommandContextRequiresCloneDir(t *testing.T) {
	cfg := &config.Config{}
	if _, err := buildOpenCommandContext(cfg, "acme/widgets", ""); err == nil {
		t.Fatal("expected error when clone_dir is empty")
	}
}

func TestBuildOpenCommandContextRejectsInvalidRepo(t *testing.T) {
	cfg := &config.Config{
		Git: config.GitConfig{
			CloneDir: "/tmp/repos",
		},
	}
	if _, err := buildOpenCommandContext(cfg, "invalid", ""); err == nil {
		t.Fatal("expected error for invalid repoFullName")
	}
}

func TestBuildOpenCommandContextDefaultsToDefaultBranchWorktree(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	cloneDir := filepath.Join(root, "repos")
	repoRoot := filepath.Join(cloneDir, "acme", "widgets")

	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "--bare", filepath.Join(repoRoot, ".git")).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v: %s", err, out)
	}
	for _, dir := range []string{"main", "feat-x"} {
		if err := os.MkdirAll(filepath.Join(repoRoot, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{Git: config.GitConfig{CloneDir: cloneDir}}
	ctx, err := buildOpenCommandContext(cfg, "acme/widgets", "")
	if err != nil {
		t.Fatalf("buildOpenCommandContext() error = %v", err)
	}

	if ctx.Worktree != "main" {
		t.Fatalf("Worktree=%q, want %q", ctx.Worktree, "main")
	}
	if ctx.RepoPath != "acme/widgets/main" {
		t.Fatalf("RepoPath=%q, want %q", ctx.RepoPath, "acme/widgets/main")
	}
	wantAbsPath := filepath.Join(repoRoot, "main")
	if ctx.AbsPath != wantAbsPath {
		t.Fatalf("AbsPath=%q, want %q", ctx.AbsPath, wantAbsPath)
	}
}

func TestBuildOpenCommandContextMasterFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	cloneDir := filepath.Join(root, "repos")
	repoRoot := filepath.Join(cloneDir, "acme", "widgets")

	if err := os.MkdirAll(filepath.Join(repoRoot, "master"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "--bare", filepath.Join(repoRoot, ".git")).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v: %s", err, out)
	}

	cfg := &config.Config{Git: config.GitConfig{CloneDir: cloneDir}}
	ctx, err := buildOpenCommandContext(cfg, "acme/widgets", "")
	if err != nil {
		t.Fatalf("buildOpenCommandContext() error = %v", err)
	}
	if ctx.Worktree != "master" {
		t.Fatalf("Worktree=%q, want master", ctx.Worktree)
	}
}

func TestBuildOpenCommandContextWorktreeLayoutFallsBackToFirstDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	cloneDir := filepath.Join(root, "repos")
	repoRoot := filepath.Join(cloneDir, "acme", "widgets")

	if err := os.MkdirAll(filepath.Join(repoRoot, "develop"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "feat-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "--bare", filepath.Join(repoRoot, ".git")).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v: %s", err, out)
	}

	cfg := &config.Config{Git: config.GitConfig{CloneDir: cloneDir}}
	ctx, err := buildOpenCommandContext(cfg, "acme/widgets", "")
	if err != nil {
		t.Fatalf("buildOpenCommandContext() error = %v", err)
	}
	if ctx.Worktree != "develop" {
		t.Fatalf("Worktree=%q, want develop (first worktree dir)", ctx.Worktree)
	}
}

func TestBuildOpenCommandContextRegularCloneOpensRepoRoot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	cloneDir := filepath.Join(root, "repos")
	repoRoot := filepath.Join(cloneDir, "acme", "widgets")

	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", repoRoot).CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	cfg := &config.Config{Git: config.GitConfig{CloneDir: cloneDir}}
	ctx, err := buildOpenCommandContext(cfg, "acme/widgets", "")
	if err != nil {
		t.Fatalf("buildOpenCommandContext() error = %v", err)
	}
	if ctx.Worktree != "" {
		t.Fatalf("Worktree=%q, want empty for regular clone", ctx.Worktree)
	}
	if ctx.AbsPath != repoRoot {
		t.Fatalf("AbsPath=%q, want %q", ctx.AbsPath, repoRoot)
	}
}
