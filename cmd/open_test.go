package cmd

import "testing"

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
