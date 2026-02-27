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

func TestGetReturnsCachedOrgWhenFresh(t *testing.T) {
	c := newTestCache(t)
	c.SetTTL(time.Hour)

	repos := []github.Repo{{FullName: "acme/repo", CreatedAt: time.Now()}}
	if err := c.Set("acme", repos); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	cached, err := c.Get("acme")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(cached.Repos) != 1 {
		t.Fatalf("len(cached.Repos) = %d, want 1", len(cached.Repos))
	}
	if cached.Repos[0].FullName != "acme/repo" {
		t.Fatalf("cached.Repos[0].FullName = %q, want %q", cached.Repos[0].FullName, "acme/repo")
	}
}

func TestGetReturnsErrorWhenOrgMissing(t *testing.T) {
	c := newTestCache(t)

	if _, err := c.Get("missing"); err == nil {
		t.Fatalf("Get() expected error for missing org")
	}
}

func TestIsExpired(t *testing.T) {
	c := newTestCache(t)
	c.SetTTL(5 * time.Millisecond)

	if err := c.Set("acme", []github.Repo{{FullName: "acme/repo", CreatedAt: time.Now()}}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if c.IsExpired("acme") {
		t.Fatalf("IsExpired(acme) = true, want false")
	}

	time.Sleep(20 * time.Millisecond)

	if !c.IsExpired("acme") {
		t.Fatalf("IsExpired(acme) = false, want true")
	}

	if !c.IsExpired("missing") {
		t.Fatalf("IsExpired(missing) = false, want true")
	}
}

func TestGetAllReposDeduplicatesAcrossOrgs(t *testing.T) {
	c := newTestCache(t)
	c.SetTTL(time.Hour)

	now := time.Now()
	if err := c.Set("acme", []github.Repo{
		{FullName: "acme/shared", CreatedAt: now.Add(-time.Hour)},
		{FullName: "acme/one", CreatedAt: now.Add(-2 * time.Hour)},
	}); err != nil {
		t.Fatalf("Set(acme) error = %v", err)
	}

	if err := c.Set("tools", []github.Repo{
		{FullName: "acme/shared", CreatedAt: now},
		{FullName: "tools/two", CreatedAt: now.Add(-3 * time.Hour)},
	}); err != nil {
		t.Fatalf("Set(tools) error = %v", err)
	}

	allRepos, err := c.GetAllRepos()
	if err != nil {
		t.Fatalf("GetAllRepos() error = %v", err)
	}

	seen := make(map[string]bool)
	for _, repo := range allRepos {
		seen[repo.FullName] = true
	}
	if len(seen) != 3 {
		t.Fatalf("unique repos = %d, want 3", len(seen))
	}
	if !seen["acme/shared"] || !seen["acme/one"] || !seen["tools/two"] {
		t.Fatalf("unexpected repo set: %+v", seen)
	}
}

func TestGetAllReposSkipsExpiredOrgCaches(t *testing.T) {
	c := newTestCache(t)

	c.SetTTL(5 * time.Millisecond)
	if err := c.Set("stale", []github.Repo{{FullName: "stale/repo", CreatedAt: time.Now()}}); err != nil {
		t.Fatalf("Set(stale) error = %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	c.SetTTL(time.Hour)
	if err := c.Set("fresh", []github.Repo{{FullName: "fresh/repo", CreatedAt: time.Now()}}); err != nil {
		t.Fatalf("Set(fresh) error = %v", err)
	}

	allRepos, err := c.GetAllRepos()
	if err != nil {
		t.Fatalf("GetAllRepos() error = %v", err)
	}
	if len(allRepos) != 1 {
		t.Fatalf("len(allRepos) = %d, want 1", len(allRepos))
	}
	if allRepos[0].FullName != "fresh/repo" {
		t.Fatalf("allRepos[0].FullName = %q, want %q", allRepos[0].FullName, "fresh/repo")
	}
}

func TestGetAllReposRefreshesSnapshotAfterSet(t *testing.T) {
	c := newTestCache(t)
	c.SetTTL(time.Hour)

	if err := c.Set("acme", []github.Repo{{FullName: "acme/one", CreatedAt: time.Now()}}); err != nil {
		t.Fatalf("Set(acme) error = %v", err)
	}

	first, err := c.GetAllRepos()
	if err != nil {
		t.Fatalf("GetAllRepos() first error = %v", err)
	}
	if len(first) != 1 || first[0].FullName != "acme/one" {
		t.Fatalf("first repos = %+v, want [acme/one]", first)
	}

	if err := c.Set("acme", []github.Repo{{FullName: "acme/two", CreatedAt: time.Now()}}); err != nil {
		t.Fatalf("Set(acme second) error = %v", err)
	}

	second, err := c.GetAllRepos()
	if err != nil {
		t.Fatalf("GetAllRepos() second error = %v", err)
	}
	if len(second) != 1 || second[0].FullName != "acme/two" {
		t.Fatalf("second repos = %+v, want [acme/two]", second)
	}
}

func TestGetAllReposCachedSnapshotHonorsExpiry(t *testing.T) {
	c := newTestCache(t)
	c.SetTTL(5 * time.Millisecond)

	if err := c.Set("acme", []github.Repo{{FullName: "acme/one", CreatedAt: time.Now()}}); err != nil {
		t.Fatalf("Set(acme) error = %v", err)
	}

	first, err := c.GetAllRepos()
	if err != nil {
		t.Fatalf("GetAllRepos() first error = %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("len(first) = %d, want 1", len(first))
	}

	time.Sleep(20 * time.Millisecond)

	second, err := c.GetAllRepos()
	if err != nil {
		t.Fatalf("GetAllRepos() second error = %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("len(second) = %d, want 0", len(second))
	}
}
