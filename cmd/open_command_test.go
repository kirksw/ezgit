package cmd

import (
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
