package cmd

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/github"
)

func TestMergeReposByFullName(t *testing.T) {
	oldest := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	middle := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newest := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	existing := []github.Repo{
		{FullName: "acme/old", CreatedAt: oldest, Description: "old"},
		{FullName: "acme/shared", CreatedAt: middle, Description: "existing"},
	}
	incoming := []github.Repo{
		{FullName: "acme/new", CreatedAt: newest, Description: "new"},
		{FullName: "acme/shared", CreatedAt: middle, Description: "updated"},
	}

	merged, added := mergeReposByFullName(existing, incoming)
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if len(merged) != 3 {
		t.Fatalf("len(merged) = %d, want 3", len(merged))
	}
	if merged[0].FullName != "acme/new" {
		t.Fatalf("merged[0] = %s, want acme/new", merged[0].FullName)
	}

	var shared github.Repo
	for _, repo := range merged {
		if repo.FullName == "acme/shared" {
			shared = repo
			break
		}
	}
	if shared.Description != "updated" {
		t.Fatalf("shared.Description = %q, want %q", shared.Description, "updated")
	}
}

func TestRefreshReposIncrementallySkipsFetchWhenCacheFresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c := cache.New()
	c.SetTTL(time.Hour)

	repos := []github.Repo{
		{FullName: "acme/existing", CreatedAt: time.Now().Add(-time.Hour)},
	}
	if err := c.Set("acme", repos); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	fetchAllCalls := 0
	fetchAfterCalls := 0
	added, total, err := refreshReposIncrementally(
		c,
		"acme",
		false,
		func() ([]github.Repo, error) {
			fetchAllCalls++
			return nil, errors.New("should not fetch all")
		},
		func(createdAfter time.Time) ([]github.Repo, error) {
			fetchAfterCalls++
			return nil, errors.New("should not fetch incrementally")
		},
	)
	if err != nil {
		t.Fatalf("refreshReposIncrementally() error = %v", err)
	}
	if added != 0 {
		t.Fatalf("added = %d, want 0", added)
	}
	if total != len(repos) {
		t.Fatalf("total = %d, want %d", total, len(repos))
	}
	if fetchAllCalls != 0 {
		t.Fatalf("fetchAllCalls = %d, want 0", fetchAllCalls)
	}
	if fetchAfterCalls != 0 {
		t.Fatalf("fetchAfterCalls = %d, want 0", fetchAfterCalls)
	}

	cacheFile := filepath.Join(home, cache.CacheDir, "acme.json")
	if cacheFile == "" {
		t.Fatal("unexpected empty cache file path")
	}
}

func TestRefreshReposIncrementallyFetchesWhenCacheExpired(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c := cache.New()
	c.SetTTL(5 * time.Millisecond)

	existingCreated := time.Now().Add(-2 * time.Hour)
	if err := c.Set("acme", []github.Repo{{FullName: "acme/existing", CreatedAt: existingCreated}}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	fetchAllCalls := 0
	fetchAfterCalls := 0
	newCreated := time.Now()
	added, total, err := refreshReposIncrementally(
		c,
		"acme",
		false,
		func() ([]github.Repo, error) {
			fetchAllCalls++
			return nil, errors.New("should not fetch all when stale exists")
		},
		func(createdAfter time.Time) ([]github.Repo, error) {
			fetchAfterCalls++
			if createdAfter.Before(existingCreated) {
				t.Fatalf("createdAfter = %s, want >= %s", createdAfter, existingCreated)
			}
			return []github.Repo{{FullName: "acme/new", CreatedAt: newCreated}}, nil
		},
	)
	if err != nil {
		t.Fatalf("refreshReposIncrementally() error = %v", err)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if fetchAllCalls != 0 {
		t.Fatalf("fetchAllCalls = %d, want 0", fetchAllCalls)
	}
	if fetchAfterCalls != 1 {
		t.Fatalf("fetchAfterCalls = %d, want 1", fetchAfterCalls)
	}
}
