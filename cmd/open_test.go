package cmd

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
)

type testRepoWorktreeLister struct {
	worktrees []string
	err       error
	lastPath  string
	calls     int
}

func (l *testRepoWorktreeLister) ListWorktrees(path string) ([]string, error) {
	l.calls++
	l.lastPath = path
	if l.err != nil {
		return nil, l.err
	}
	return append([]string(nil), l.worktrees...), nil
}

func TestWithCreateWorktreeOptionAppendsCreateChoice(t *testing.T) {
	got := withCreateWorktreeOption([]string{"main", "review"})
	if len(got) != 3 {
		t.Fatalf("len(got)=%d, want 3", len(got))
	}
	if got[0] != "main" || got[1] != "review" {
		t.Fatalf("unexpected preserved options: %v", got[:2])
	}
	if got[2] != createNewWorktreeOption {
		t.Fatalf("got last option %q, want %q", got[2], createNewWorktreeOption)
	}
}

func TestIsCreateWorktreeOption(t *testing.T) {
	if !isCreateWorktreeOption(createNewWorktreeOption) {
		t.Fatal("expected create option to match")
	}
	if isCreateWorktreeOption("main") {
		t.Fatal("did not expect non-create option to match")
	}
}

func TestContainsString(t *testing.T) {
	if !containsString([]string{"main", "review"}, "review") {
		t.Fatal("expected containsString to find value")
	}
	if containsString([]string{"main", "review"}, "feature/test") {
		t.Fatal("did not expect containsString to find missing value")
	}
}

func TestRunOpenRequiresWorktreeWhenRepoArgProvided(t *testing.T) {
	err := runOpen(nil, []string{"lunarway/hubble-cli"})
	if err == nil {
		t.Fatal("expected usage error")
	}
	if err.Error() != "usage: ezgit open <repo> <worktree-name>" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildOpenWorktreeLoaderLoadsLocalRepoWorktrees(t *testing.T) {
	cfg := &config.Config{Git: config.GitConfig{CloneDir: "/tmp/clones"}}
	localRepos := map[string]bool{"acme/widgets": true}
	lister := &testRepoWorktreeLister{worktrees: []string{"main", "review"}}

	loader := buildOpenWorktreeLoader(cfg, localRepos, lister)
	if loader == nil {
		t.Fatal("expected non-nil loader")
	}

	worktrees, err := loader(github.Repo{FullName: "acme/widgets", DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if len(worktrees) != 2 {
		t.Fatalf("len(worktrees) = %d, want 2", len(worktrees))
	}
	if worktrees[0] != "main" || worktrees[1] != "review" {
		t.Fatalf("worktrees = %v, want [main review]", worktrees)
	}

	wantPath := filepath.Join("/tmp/clones", "acme", "widgets")
	if lister.lastPath != wantPath {
		t.Fatalf("lister path = %q, want %q", lister.lastPath, wantPath)
	}
	if lister.calls != 1 {
		t.Fatalf("lister calls = %d, want 1", lister.calls)
	}
}

func TestBuildOpenWorktreeLoaderSkipsNonLocalRepo(t *testing.T) {
	cfg := &config.Config{Git: config.GitConfig{CloneDir: "/tmp/clones"}}
	lister := &testRepoWorktreeLister{worktrees: []string{"main"}}

	loader := buildOpenWorktreeLoader(cfg, map[string]bool{}, lister)
	if loader == nil {
		t.Fatal("expected non-nil loader")
	}

	worktrees, err := loader(github.Repo{FullName: "acme/widgets", DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if len(worktrees) != 0 {
		t.Fatalf("len(worktrees) = %d, want 0", len(worktrees))
	}
	if lister.calls != 0 {
		t.Fatalf("lister calls = %d, want 0", lister.calls)
	}
}

func TestBuildOpenWorktreeLoaderPropagatesListerError(t *testing.T) {
	cfg := &config.Config{Git: config.GitConfig{CloneDir: "/tmp/clones"}}
	localRepos := map[string]bool{"acme/widgets": true}
	lister := &testRepoWorktreeLister{err: fmt.Errorf("boom")}

	loader := buildOpenWorktreeLoader(cfg, localRepos, lister)
	_, err := loader(github.Repo{FullName: "acme/widgets", DefaultBranch: "main"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "boom" {
		t.Fatalf("error = %v, want boom", err)
	}
}
