package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kirksw/ezgit/internal/git"
)

func TestFormatWorktreeAgeOmitsZeroParts(t *testing.T) {
	for _, test := range []struct {
		duration time.Duration
		want     string
	}{
		{duration: 14 * 24 * time.Hour, want: "14d"},
		{duration: 14*24*time.Hour + 3*time.Hour, want: "14d 3h"},
		{duration: 14*24*time.Hour + 3*time.Hour + 12*time.Minute, want: "14d 3h 12m"},
		{duration: 45 * time.Minute, want: "45m"},
	} {
		if got := formatWorktreeAge(test.duration); got != test.want {
			t.Fatalf("formatWorktreeAge(%v) = %q, want %q", test.duration, got, test.want)
		}
	}
}

func TestParseWorktreeAgeSupportsDaysAndHours(t *testing.T) {
	for _, test := range []struct {
		input string
		want  time.Duration
	}{
		{input: "14d", want: 14 * 24 * time.Hour},
		{input: "48h", want: 48 * time.Hour},
	} {
		got, err := parseWorktreeAge(test.input)
		if err != nil {
			t.Fatalf("parseWorktreeAge(%q) error = %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf("parseWorktreeAge(%q) = %v, want %v", test.input, got, test.want)
		}
	}
}

func TestParseWorktreeAgeRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{"", "0d", "two-weeks"} {
		if _, err := parseWorktreeAge(input); err == nil {
			t.Fatalf("parseWorktreeAge(%q) returned nil error", input)
		}
	}
}

func TestFindStaleWorktreesUsesNewestFileModificationAndProtectsMainMaster(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-20 * 24 * time.Hour).Truncate(time.Second)
	newer := time.Now().Add(-2 * 24 * time.Hour).Truncate(time.Second)

	makeTree := func(name string, modified time.Time) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(path, "file.txt")
		if err := os.WriteFile(file, []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, modified, modified); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
		return path
	}

	worktrees := []git.Worktree{
		{Path: makeTree("main", old), Branch: "main"},
		{Path: makeTree("master", old), Branch: "master"},
		{Path: makeTree("old-feature", old), Branch: "old-feature"},
		{Path: makeTree("recent-feature", newer), Branch: "recent-feature"},
	}
	candidates, err := findStaleWorktrees(worktrees, time.Now().Add(-14*24*time.Hour))
	if err != nil {
		t.Fatalf("findStaleWorktrees() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Branch != "old-feature" {
		t.Fatalf("findStaleWorktrees() = %+v, want old-feature only", candidates)
	}
}

func TestNewestWorktreeModificationIgnoresGitMetadata(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-20 * 24 * time.Hour).Truncate(time.Second)
	recent := time.Now().Truncate(time.Second)

	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	gitFile := filepath.Join(root, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: elsewhere"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(gitFile, recent, recent); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(root, old, old); err != nil {
		t.Fatal(err)
	}

	got, err := newestWorktreeModification(root)
	if err != nil {
		t.Fatalf("newestWorktreeModification() error = %v", err)
	}
	if !got.Equal(old) {
		t.Fatalf("newest modification = %v, want %v", got, old)
	}
}
