package cache

import (
	"os"
	"testing"
	"time"

	"github.com/kirksw/ezgit/internal/github"
)

func newTestCache(t *testing.T) *OrgCache {
	t.Helper()
	return &OrgCache{
		cacheDir: t.TempDir(),
		ttl:      DefaultTTL,
	}
}

func TestSetSortsReposByCreatedAtAndStoresLatestDate(t *testing.T) {
	c := newTestCache(t)

	oldest := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	newest := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	repos := []github.Repo{
		{FullName: "acme/old", CreatedAt: oldest},
		{FullName: "acme/new", CreatedAt: newest},
	}

	if err := c.Set("acme", repos); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	stale, err := c.GetStale("acme")
	if err != nil {
		t.Fatalf("GetStale() error = %v", err)
	}
	if len(stale.Repos) != 2 {
		t.Fatalf("len(repos) = %d, want 2", len(stale.Repos))
	}
	if stale.Repos[0].FullName != "acme/new" {
		t.Fatalf("repos not sorted by created_at desc: first = %s", stale.Repos[0].FullName)
	}

	latest, err := c.GetLatestRepoCreatedAt("acme")
	if err != nil {
		t.Fatalf("GetLatestRepoCreatedAt() error = %v", err)
	}
	if !latest.Equal(newest) {
		t.Fatalf("latest = %s, want %s", latest, newest)
	}
}

func TestGetLatestRepoCreatedAtFallsBackWhenMetadataMissing(t *testing.T) {
	c := newTestCache(t)

	created := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := c.Set("acme", []github.Repo{{FullName: "acme/repo", CreatedAt: created}}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := os.Remove(c.metadataPath("acme")); err != nil {
		t.Fatalf("failed to remove metadata: %v", err)
	}

	latest, err := c.GetLatestRepoCreatedAt("acme")
	if err != nil {
		t.Fatalf("GetLatestRepoCreatedAt() error = %v", err)
	}
	if !latest.Equal(created) {
		t.Fatalf("latest = %s, want %s", latest, created)
	}
}
