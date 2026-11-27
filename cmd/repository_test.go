package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
)

func TestParseRepository(t *testing.T) {
	for _, tc := range []struct{ input, host, url string }{
		{"acme/repo", "github.com", "git@github.com:acme/repo.git"},
		{"git.example.com/acme/repo", "git.example.com", "git@git.example.com:acme/repo.git"},
		{"https://Git.Example.com/acme/repo", "git.example.com", "https://Git.Example.com/acme/repo.git"},
		{"git@git.example.com:acme/repo.git", "git.example.com", "git@git.example.com:acme/repo.git"},
		{"ssh://git@git.example.com:22/acme/repo.git", "git.example.com", "ssh://git@git.example.com:22/acme/repo.git"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			repo, err := parseRepository(tc.input)
			if err != nil || repo.Host != tc.host || repo.fullName() != "acme/repo" || repo.URL != tc.url {
				t.Fatalf("parseRepository = %+v, %v", repo, err)
			}
		})
	}
	for _, input := range []string{"ssh://git@git.example.com:2222/acme/repo.git", "../acme/repo", "github.com/../repo", ".worktrees/acme/repo", "acme/..", "git@git.example.com:/acme/repo", "https://git.example.com/acme/%2e%2e", "https://credential@git.example.com/acme/repo", "https://git.example.com/acme/repo?q=1", "file:///tmp/acme/repo"} {
		if _, err := parseRepository(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestRepositoryPathPrefersCanonicalButRecognizesConfiguredLegacy(t *testing.T) {
	cfg := &config.Config{Git: config.GitConfig{Root: t.TempDir(), CloneDir: t.TempDir()}}
	repo, _ := parseRepository("acme/repo")
	canonical := filepath.Join(cfg.Git.Root, "github.com", "acme", "repo")
	legacy := filepath.Join(cfg.Git.CloneDir, "acme", "repo")
	if got := repo.repoPath(cfg); got != canonical {
		t.Fatalf("new clone path = %q, want %q", got, canonical)
	}
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatal(err)
	}
	if got := repo.repoPath(cfg); got != legacy {
		t.Fatalf("legacy clone path = %q, want %q", got, legacy)
	}
	if err := os.MkdirAll(canonical, 0755); err != nil {
		t.Fatal(err)
	}
	if got := repo.repoPath(cfg); got != canonical {
		t.Fatalf("canonical clone path = %q, want %q", got, canonical)
	}
}

func TestAgentWorktreesShareExplicitBaseWithoutChangingPrimary(t *testing.T) {
	cfg, _ := newSafetyEnv(t)
	cfg.Git = config.GitConfig{Root: t.TempDir()}
	repo, err := parseRepository("github.com/acme/repo")
	if err != nil {
		t.Fatal(err)
	}
	primary := repo.repoPath(cfg)
	cloneSafetyRepo(t, cfg, primary)
	base := runSafetyGit(t, primary, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(primary, "untracked"), []byte("keep\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"agent-code", "agent-test"} {
		result, err := createLinkedWorktree(cfg, repo, name, base)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(cfg.Git.Root, ".worktrees", "github.com", "acme", "repo", name)
		if result != (worktreeResult{Path: want, Branch: name, BaseCommit: base}) {
			t.Fatalf("result = %+v", result)
		}
		if got := runSafetyGit(t, result.Path, "rev-parse", "HEAD"); got != base {
			t.Fatalf("checkout HEAD = %q, want %q", got, base)
		}
	}
	if _, err := createLinkedWorktree(cfg, repo, "agent-code", base); err == nil {
		t.Fatal("an existing worktree must not be assigned to a second writer")
	}
	if _, err := createLinkedWorktree(cfg, repo, "invalid-base", "does-not-exist"); err == nil {
		t.Fatal("an invalid base must not create a worktree")
	}
	if _, err := createLinkedWorktree(cfg, repo, "../escape", base); err == nil {
		t.Fatal("a path escape must be rejected")
	}
	if got := runSafetyGit(t, primary, "status", "--porcelain"); got != "?? untracked" {
		t.Fatalf("primary status = %q, want untouched untracked file", got)
	}
	if got := runSafetyGit(t, primary, "branch", "--show-current"); got != "main" {
		t.Fatalf("primary branch = %q, want main", got)
	}
	desc, err := describeRepo(cfg, "github.com/acme/repo", git.New())
	if err != nil || desc.Host != "github.com" || desc.Layout != "regular" || strings.Join(desc.Worktrees, ",") != "agent-code,agent-test" {
		t.Fatalf("description = %+v, %v", desc, err)
	}
}

func TestOpenDisambiguatesWorktreesWithTheSameName(t *testing.T) {
	cfg, primary := newSafetyEnv(t)
	cloneSafetyRepo(t, cfg, primary)
	for _, dir := range []string{"team-a", "team-b"} {
		path := filepath.Join(t.TempDir(), dir, "review")
		runSafetyGit(t, primary, "worktree", "add", "--detach", path, "HEAD")
	}
	selectors, err := git.New().ListWorktrees(primary)
	if err != nil || len(selectors) != 2 {
		t.Fatalf("selectors = %v, %v", selectors, err)
	}
	if _, err := buildOpenCommandContext(cfg, "acme/repo", "review"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("short-name lookup: %v", err)
	}
	for _, path := range selectors {
		ctx, err := buildOpenCommandContext(cfg, "acme/repo", path)
		if err != nil || !filepath.IsAbs(path) || ctx.AbsPath != path {
			t.Fatalf("opening %q: context = %+v, error = %v", path, ctx, err)
		}
	}
}

func TestPruneApplyKeepsPrimaryAndWorktreeContainingIgnoredFile(t *testing.T) {
	cfg, primary := newSafetyEnv(t)
	cloneSafetyRepo(t, cfg, primary)
	repo, _ := parseRepository("acme/repo")
	ignored, err := createLinkedWorktree(cfg, repo, "ignored", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	clean, err := createLinkedWorktree(cfg, repo, "clean", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	runSafetyGit(t, primary, "config", "core.excludesfile", filepath.Join(t.TempDir(), "ignores"))
	excludes := runSafetyGit(t, primary, "config", "core.excludesfile")
	if err := os.WriteFile(excludes, []byte("precious\n"), 0644); err != nil {
		t.Fatal(err)
	}
	precious := filepath.Join(ignored.Path, "precious")
	if err := os.WriteFile(precious, []byte("keep\n"), 0644); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configFile, []byte("[git]\nclone_dir = \""+cfg.Git.CloneDir+"\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	prevConfig, prevAge, prevApply := configPath, worktreePruneOlderThan, worktreePruneApply
	t.Cleanup(func() { configPath, worktreePruneOlderThan, worktreePruneApply = prevConfig, prevAge, prevApply })
	configPath, worktreePruneOlderThan, worktreePruneApply = configFile, "1ns", true
	if err := runWorktreePrune(worktreePruneCmd, []string{"acme/repo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(clean.Path); !os.IsNotExist(err) {
		t.Fatalf("clean worktree still exists: %v", err)
	}
	if _, err := os.Stat(precious); err != nil {
		t.Fatalf("ignored file was not preserved: %v", err)
	}
	if _, err := os.Stat(primary); err != nil {
		t.Fatalf("primary checkout was not preserved: %v", err)
	}
}
