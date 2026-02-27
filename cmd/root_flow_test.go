package cmd

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
)

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

func TestBuildOpenedRepoMapFromSessions(t *testing.T) {
	allRepos := []github.Repo{
		{FullName: "acme/api"},
		{FullName: "acme/web"},
		{FullName: "tools/cli"},
	}
	sessions := []string{
		"acme/api/main",
		"acme-web/review",
		"/Users/me/git/github.com/tools/cli",
		"",
	}

	got := buildOpenedRepoMapFromSessions(allRepos, sessions)

	if !got["acme/api"] {
		t.Fatalf("expected acme/api to be marked opened")
	}
	if !got["acme/web"] {
		t.Fatalf("expected acme/web to be marked opened")
	}
	if !got["tools/cli"] {
		t.Fatalf("expected tools/cli to be marked opened")
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
}

func TestBuildOpenedWorktreeMapFromSessions(t *testing.T) {
	allRepos := []github.Repo{
		{FullName: "acme/api"},
		{FullName: "acme/web"},
	}
	sessions := []string{
		"acme/api/main",
		"acme-api/review",
		"/Users/me/git/github.com/acme/web/feature/x",
		"acme/web",
	}

	got := buildOpenedWorktreeMapFromSessions(allRepos, sessions)

	if !got["acme/api"]["main"] {
		t.Fatalf("expected acme/api main to be opened")
	}
	if !got["acme/api"]["review"] {
		t.Fatalf("expected acme/api review to be opened")
	}
	if !got["acme/web"]["feature/x"] {
		t.Fatalf("expected acme/web feature/x to be opened")
	}
	if _, ok := got["acme/web"][""]; ok {
		t.Fatalf("did not expect empty worktree entry")
	}
}

type trackingWorktreeLister struct {
	delay time.Duration

	mu          sync.Mutex
	inFlight    int
	maxInFlight int
	callCount   int
}

func (l *trackingWorktreeLister) ListWorktrees(path string) ([]string, error) {
	l.mu.Lock()
	l.inFlight++
	if l.inFlight > l.maxInFlight {
		l.maxInFlight = l.inFlight
	}
	l.callCount++
	l.mu.Unlock()

	time.Sleep(l.delay)

	l.mu.Lock()
	l.inFlight--
	l.mu.Unlock()

	return []string{fmt.Sprintf("%s-wt", filepath.Base(path))}, nil
}

func TestBuildLocalRepoWorktreeMapWithListerProcessesConcurrently(t *testing.T) {
	cloneDir := t.TempDir()
	cfg := &config.Config{Git: config.GitConfig{CloneDir: cloneDir}}

	allRepos := []github.Repo{
		{FullName: "acme/repo1"},
		{FullName: "acme/repo2"},
		{FullName: "acme/repo3"},
		{FullName: "acme/repo4"},
		{FullName: "acme/repo5"},
		{FullName: "acme/repo6"},
	}
	localRepos := map[string]bool{
		"acme/repo1": true,
		"acme/repo2": true,
		"acme/repo3": true,
		"acme/repo4": true,
		"acme/repo5": true,
		"acme/repo6": true,
	}

	lister := &trackingWorktreeLister{delay: 25 * time.Millisecond}
	got := buildLocalRepoWorktreeMapWithLister(cfg, allRepos, localRepos, lister, 4)

	if len(got) != len(allRepos) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(allRepos))
	}

	for _, repo := range allRepos {
		worktrees, ok := got[repo.FullName]
		if !ok {
			t.Fatalf("missing worktrees for %s", repo.FullName)
		}
		if len(worktrees) != 1 {
			t.Fatalf("len(worktrees[%s]) = %d, want 1", repo.FullName, len(worktrees))
		}
	}

	if lister.callCount != len(allRepos) {
		t.Fatalf("callCount = %d, want %d", lister.callCount, len(allRepos))
	}
	if lister.maxInFlight <= 1 {
		t.Fatalf("maxInFlight = %d, want > 1", lister.maxInFlight)
	}
}
