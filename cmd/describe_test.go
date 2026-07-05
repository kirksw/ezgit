package cmd

import (
	"path/filepath"
	"testing"

	"github.com/kirksw/ezgit/internal/config"
)

func TestRepoLayout(t *testing.T) {
	cases := map[existingRepoState]string{
		existingRepoMissing:  "missing",
		existingRepoRegular:  "regular",
		existingRepoWorktree: "worktree",
		existingRepoNonRepo:  "non_repo",
	}
	for state, want := range cases {
		if got := repoLayout(state); got != want {
			t.Fatalf("repoLayout(%v) = %q, want %q", state, got, want)
		}
	}
}

func TestDescribeRepoMissing(t *testing.T) {
	cloneDir := t.TempDir()
	cfg := &config.Config{Git: config.GitConfig{CloneDir: cloneDir}}

	desc, err := describeRepo(cfg, "acme/widgets", nil)
	if err != nil {
		t.Fatalf("describeRepo() error = %v", err)
	}
	if desc.Cloned || desc.Worktree || desc.Layout != "missing" {
		t.Fatalf("unexpected desc: %+v", desc)
	}
	wantPath := filepath.Join(cloneDir, "acme", "widgets")
	if desc.Path != wantPath {
		t.Fatalf("Path = %q, want %q", desc.Path, wantPath)
	}
}
