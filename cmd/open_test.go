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

func TestContainsString(t *testing.T) {
	if !containsString([]string{"main", "review"}, "review") {
		t.Fatal("expected containsString to find value")
	}
	if containsString([]string{"main", "review"}, "feature/test") {
		t.Fatal("did not expect containsString to find missing value")
	}
}

func TestIsBuiltInWorktree(t *testing.T) {
	if !isBuiltInWorktree("main", "main") || !isBuiltInWorktree("review", "main") {
		t.Fatal("expected main/review to be built-in worktrees")
	}
	if isBuiltInWorktree("feature", "main") {
		t.Fatal("did not expect feature to be built-in")
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
