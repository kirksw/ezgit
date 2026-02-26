package cmd

import "testing"

func TestSessionMatchesRepo(t *testing.T) {
	tests := []struct {
		name     string
		session  string
		repo     string
		expected bool
	}{
		{name: "exact full name", session: "org/repo", repo: "org/repo", expected: true},
		{name: "full name with worktree", session: "org/repo/main", repo: "org/repo", expected: true},
		{name: "normalized exact", session: "org-repo", repo: "org/repo", expected: true},
		{name: "normalized with worktree", session: "org-repo/review", repo: "org/repo", expected: true},
		{name: "path contains full name", session: "/Users/me/git/github.com/org/repo/main", repo: "org/repo", expected: true},
		{name: "different repo", session: "other/repo", repo: "org/repo", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sessionMatchesRepo(tt.session, tt.repo)
			if got != tt.expected {
				t.Fatalf("sessionMatchesRepo(%q,%q)=%v, want %v", tt.session, tt.repo, got, tt.expected)
			}
		})
	}
}

func TestExtractWorktreeFromSession(t *testing.T) {
	tests := []struct {
		name         string
		session      string
		repo         string
		wantWorktree string
		wantOK       bool
	}{
		{name: "full name worktree", session: "org/repo/main", repo: "org/repo", wantWorktree: "main", wantOK: true},
		{name: "normalized worktree", session: "org-repo/review", repo: "org/repo", wantWorktree: "review", wantOK: true},
		{name: "path worktree", session: "/Users/me/git/github.com/org/repo/feature/x", repo: "org/repo", wantWorktree: "feature/x", wantOK: true},
		{name: "repo root only", session: "org/repo", repo: "org/repo", wantWorktree: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWorktree, gotOK := extractWorktreeFromSession(tt.session, tt.repo)
			if gotOK != tt.wantOK {
				t.Fatalf("extractWorktreeFromSession(%q,%q) ok=%v, want %v", tt.session, tt.repo, gotOK, tt.wantOK)
			}
			if gotWorktree != tt.wantWorktree {
				t.Fatalf("extractWorktreeFromSession(%q,%q) worktree=%q, want %q", tt.session, tt.repo, gotWorktree, tt.wantWorktree)
			}
		})
	}
}
