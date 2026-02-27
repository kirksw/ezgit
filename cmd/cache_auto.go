package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
)

type cacheAutoGitHubClient interface {
	ValidateToken() error
	FetchOrgRepos(org string) ([]github.Repo, error)
	FetchOrgReposCreatedAfter(org string, createdAfter time.Time) ([]github.Repo, error)
	FetchPrivateRepos() ([]github.Repo, error)
	FetchPrivateReposCreatedAfter(createdAfter time.Time) ([]github.Repo, error)
}

var (
	getGitHubTokenForAutoRefresh = func(cfg *config.Config) string {
		return cfg.GetGitHubToken()
	}
	newCacheAutoGitHubClient = func(token string) cacheAutoGitHubClient {
		return github.NewClient(token)
	}
	refreshReposIncrementallyForAuto = refreshReposIncrementally
)

// autoRefreshConfiguredCaches performs the same incremental refresh strategy as
// `ezgit cache refresh` for configured orgs + personal repos, but is intended
// for internal command flows (clone/open) and should not block command usage on
// partial refresh failures.
func autoRefreshConfiguredCaches(cfg *config.Config, c *cache.OrgCache) error {
	refreshTargets := make([]string, 0, len(cfg.GetOrganizations())+1)
	for _, org := range cfg.GetOrganizations() {
		if c.IsExpired(org) {
			refreshTargets = append(refreshTargets, org)
		}
	}
	if c.IsExpired(cache.PersonalCacheKey) {
		refreshTargets = append(refreshTargets, cache.PersonalCacheKey)
	}

	if len(refreshTargets) == 0 {
		return nil
	}

	token := getGitHubTokenForAutoRefresh(cfg)
	if token == "" {
		return nil
	}

	client := newCacheAutoGitHubClient(token)
	if err := client.ValidateToken(); err != nil {
		return fmt.Errorf("invalid GitHub token: %w", err)
	}

	var failures []string
	var failuresMu sync.Mutex
	var refreshWg sync.WaitGroup

	for _, target := range refreshTargets {
		if target == cache.PersonalCacheKey {
			continue
		}

		org := target
		refreshWg.Add(1)
		go func() {
			defer refreshWg.Done()
			if _, _, err := refreshReposIncrementallyForAuto(
				c,
				org,
				false,
				func() ([]github.Repo, error) {
					return client.FetchOrgRepos(org)
				},
				func(createdAfter time.Time) ([]github.Repo, error) {
					return client.FetchOrgReposCreatedAfter(org, createdAfter)
				},
			); err != nil {
				failuresMu.Lock()
				failures = append(failures, fmt.Sprintf("%s: %v", org, err))
				failuresMu.Unlock()
			}
		}()
	}

	for _, target := range refreshTargets {
		if target != cache.PersonalCacheKey {
			continue
		}

		refreshWg.Add(1)
		go func() {
			defer refreshWg.Done()
			if _, _, err := refreshReposIncrementallyForAuto(
				c,
				cache.PersonalCacheKey,
				false,
				func() ([]github.Repo, error) {
					return client.FetchPrivateRepos()
				},
				func(createdAfter time.Time) ([]github.Repo, error) {
					return client.FetchPrivateReposCreatedAfter(createdAfter)
				},
			); err != nil {
				failuresMu.Lock()
				failures = append(failures, fmt.Sprintf("%s: %v", cache.PersonalCacheKey, err))
				failuresMu.Unlock()
			}
		}()
	}

	refreshWg.Wait()

	if len(failures) > 0 {
		sort.Strings(failures)
		return errors.New(strings.Join(failures, "; "))
	}

	return nil
}

func startAutoRefreshConfiguredCaches(cfg *config.Config, c *cache.OrgCache) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- autoRefreshConfiguredCaches(cfg, c)
		close(done)
	}()
	return done
}

func warnIfBackgroundAutoRefreshFailed(done <-chan error) {
	if done == nil {
		return
	}

	select {
	case err, ok := <-done:
		if ok && err != nil {
			fmt.Printf("Warning: automatic cache refresh failed: %v\n", err)
		}
	default:
	}
}
