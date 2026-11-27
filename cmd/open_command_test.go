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
	cfg, repoPath := newSafetyEnv(t)
	cloneSafetyRepo(t, cfg, repoPath)
	worktreePath := filepath.Join(t.TempDir(), "external", "review")
	runSafetyGit(t, repoPath, "worktree", "add", "--detach", worktreePath, "HEAD")
	worktreePath, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		t.Fatal(err)
	}

	ctx, err := buildOpenCommandContext(cfg, "acme/repo", "review")
	if err != nil {
		t.Fatalf("buildOpenCommandContext() error = %v", err)
	}

	if ctx.Org != "acme" {
		t.Fatalf("Org=%q, want %q", ctx.Org, "acme")
	}
	if ctx.Repo != "repo" {
		t.Fatalf("Repo=%q, want %q", ctx.Repo, "repo")
	}
	if ctx.Worktree != "review" {
		t.Fatalf("Worktree=%q, want %q", ctx.Worktree, "review")
	}
	if ctx.OrgRepo != "acme/repo" {
		t.Fatalf("OrgRepo=%q, want %q", ctx.OrgRepo, "acme/repo")
	}
	if ctx.RepoPath != "acme/repo/review" {
		t.Fatalf("RepoPath=%q, want %q", ctx.RepoPath, "acme/repo/review")
	}
	wantAbsPath := worktreePath
	if ctx.AbsPath != wantAbsPath {
		t.Fatalf("AbsPath=%q, want %q", ctx.AbsPath, wantAbsPath)
	}
}

func TestBuildOpenCommandContextRequiresLocalCheckout(t *testing.T) {
	cfg := &config.Config{Git: config.GitConfig{Root: t.TempDir()}}
	if _, err := buildOpenCommandContext(cfg, "acme/widgets", ""); err == nil {
		t.Fatal("expected error when repository has not been cloned")
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

func TestBuildOpenCommandContextOpensRegisteredLegacyCheckout(t *testing.T) {
	for _, tc := range []struct{ branch, name string }{{"main", "main"}, {"master", "master"}, {"main", "feature"}} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, repoRoot := newSafetyEnv(t)
			source := filepath.Join(t.TempDir(), "source")
			runSafetyGit(t, "", "init", "-b", tc.branch, source)
			runSafetyGit(t, source, "commit", "--allow-empty", "-m", "initial")
			runSafetyGit(t, source, "clone", "--bare", source, filepath.Join(repoRoot, ".git"))
			if err := os.MkdirAll(filepath.Join(repoRoot, "unregistered"), 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := buildOpenCommandContext(cfg, "acme/repo", ""); err == nil {
				t.Fatal("bare metadata is not a checkout")
			}
			runSafetyGit(t, filepath.Join(repoRoot, ".git"), "worktree", "add", "--detach", filepath.Join(repoRoot, tc.name), tc.branch)
			ctx, err := buildOpenCommandContext(cfg, "acme/repo", "")
			if err != nil || ctx.Worktree != tc.name {
				t.Fatalf("context = %+v, err = %v, want %s", ctx, err, tc.name)
			}
		})
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
