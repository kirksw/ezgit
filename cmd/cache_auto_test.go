package cmd

import (
	"sync"
	"testing"
	"time"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
)

type fakeCacheAutoGitHubClient struct {
	validateCalls int
}

func (f *fakeCacheAutoGitHubClient) ValidateToken() error {
	f.validateCalls++
	return nil
}

func (f *fakeCacheAutoGitHubClient) FetchOrgRepos(org string) ([]github.Repo, error) {
	return nil, nil
}

func (f *fakeCacheAutoGitHubClient) FetchOrgReposCreatedAfter(org string, createdAfter time.Time) ([]github.Repo, error) {
	return nil, nil
}

func (f *fakeCacheAutoGitHubClient) FetchPrivateRepos() ([]github.Repo, error) {
	return nil, nil
}

func (f *fakeCacheAutoGitHubClient) FetchPrivateReposCreatedAfter(createdAfter time.Time) ([]github.Repo, error) {
	return nil, nil
}

func TestAutoRefreshConfiguredCachesSkipsWithoutToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := cache.New()
	cfg := &config.Config{}

	originalGetToken := getGitHubTokenForAutoRefresh
	originalNewClient := newCacheAutoGitHubClient
	originalRefresh := refreshReposIncrementallyForAuto
	defer func() {
		getGitHubTokenForAutoRefresh = originalGetToken
		newCacheAutoGitHubClient = originalNewClient
		refreshReposIncrementallyForAuto = originalRefresh
	}()

	clientConstructed := false
	getGitHubTokenForAutoRefresh = func(cfg *config.Config) string {
		return ""
	}
	newCacheAutoGitHubClient = func(token string) cacheAutoGitHubClient {
		clientConstructed = true
		return &fakeCacheAutoGitHubClient{}
	}

	if err := autoRefreshConfiguredCaches(cfg, c); err != nil {
		t.Fatalf("autoRefreshConfiguredCaches() error = %v", err)
	}
	if clientConstructed {
		t.Fatalf("expected no GitHub client to be created when token is missing")
	}
}

func TestAutoRefreshConfiguredCachesRefreshesTargetsConcurrently(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := cache.New()
	cfg := &config.Config{Organizations: config.OrganizationConfig{Orgs: []string{"acme", "tools", "infra"}}}

	originalGetToken := getGitHubTokenForAutoRefresh
	originalNewClient := newCacheAutoGitHubClient
	originalRefresh := refreshReposIncrementallyForAuto
	defer func() {
		getGitHubTokenForAutoRefresh = originalGetToken
		newCacheAutoGitHubClient = originalNewClient
		refreshReposIncrementallyForAuto = originalRefresh
	}()

	fakeClient := &fakeCacheAutoGitHubClient{}
	getGitHubTokenForAutoRefresh = func(cfg *config.Config) string {
		return "token"
	}
	newCacheAutoGitHubClient = func(token string) cacheAutoGitHubClient {
		return fakeClient
	}

	seen := make(map[string]int)
	active := 0
	maxActive := 0
	var mu sync.Mutex

	refreshReposIncrementallyForAuto = func(
		c *cache.OrgCache,
		cacheKey string,
		fullRefresh bool,
		fetchAll func() ([]github.Repo, error),
		fetchCreatedAfter func(time.Time) ([]github.Repo, error),
	) (added int, total int, err error) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		seen[cacheKey]++
		mu.Unlock()

		time.Sleep(30 * time.Millisecond)

		mu.Lock()
		active--
		mu.Unlock()

		return 0, 0, nil
	}

	if err := autoRefreshConfiguredCaches(cfg, c); err != nil {
		t.Fatalf("autoRefreshConfiguredCaches() error = %v", err)
	}

	if fakeClient.validateCalls != 1 {
		t.Fatalf("ValidateToken calls = %d, want 1", fakeClient.validateCalls)
	}

	wantCalls := len(cfg.GetOrganizations()) + 1
	totalCalls := 0
	for _, count := range seen {
		totalCalls += count
	}
	if totalCalls != wantCalls {
		t.Fatalf("refresh calls = %d, want %d", totalCalls, wantCalls)
	}

	for _, org := range cfg.GetOrganizations() {
		if seen[org] != 1 {
			t.Fatalf("refresh count for %s = %d, want 1", org, seen[org])
		}
	}
	if seen[cache.PersonalCacheKey] != 1 {
		t.Fatalf("refresh count for %s = %d, want 1", cache.PersonalCacheKey, seen[cache.PersonalCacheKey])
	}
	if maxActive <= 1 {
		t.Fatalf("maxActive = %d, want > 1", maxActive)
	}
}

func TestAutoRefreshConfiguredCachesSkipsEverythingWhenAllCachesFresh(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := cache.New()
	c.SetTTL(time.Hour)

	now := time.Now()
	if err := c.Set("acme", []github.Repo{{FullName: "acme/repo", CreatedAt: now}}); err != nil {
		t.Fatalf("Set(acme) error = %v", err)
	}
	if err := c.Set("tools", []github.Repo{{FullName: "tools/repo", CreatedAt: now}}); err != nil {
		t.Fatalf("Set(tools) error = %v", err)
	}
	if err := c.Set(cache.PersonalCacheKey, []github.Repo{{FullName: "me/repo", CreatedAt: now}}); err != nil {
		t.Fatalf("Set(personal) error = %v", err)
	}

	cfg := &config.Config{Organizations: config.OrganizationConfig{Orgs: []string{"acme", "tools"}}}

	originalGetToken := getGitHubTokenForAutoRefresh
	originalNewClient := newCacheAutoGitHubClient
	originalRefresh := refreshReposIncrementallyForAuto
	defer func() {
		getGitHubTokenForAutoRefresh = originalGetToken
		newCacheAutoGitHubClient = originalNewClient
		refreshReposIncrementallyForAuto = originalRefresh
	}()

	getTokenCalls := 0
	newClientCalls := 0
	refreshCalls := 0

	getGitHubTokenForAutoRefresh = func(cfg *config.Config) string {
		getTokenCalls++
		return "token"
	}
	newCacheAutoGitHubClient = func(token string) cacheAutoGitHubClient {
		newClientCalls++
		return &fakeCacheAutoGitHubClient{}
	}
	refreshReposIncrementallyForAuto = func(
		c *cache.OrgCache,
		cacheKey string,
		fullRefresh bool,
		fetchAll func() ([]github.Repo, error),
		fetchCreatedAfter func(time.Time) ([]github.Repo, error),
	) (added int, total int, err error) {
		refreshCalls++
		return 0, 0, nil
	}

	if err := autoRefreshConfiguredCaches(cfg, c); err != nil {
		t.Fatalf("autoRefreshConfiguredCaches() error = %v", err)
	}

	if getTokenCalls != 0 {
		t.Fatalf("get token calls = %d, want 0", getTokenCalls)
	}
	if newClientCalls != 0 {
		t.Fatalf("new client calls = %d, want 0", newClientCalls)
	}
	if refreshCalls != 0 {
		t.Fatalf("refresh calls = %d, want 0", refreshCalls)
	}
}
