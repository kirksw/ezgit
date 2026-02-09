package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
)

// autoRefreshConfiguredCaches performs the same incremental refresh strategy as
// `ezgit cache refresh` for configured orgs + personal repos, but is intended
// for internal command flows (clone/open) and should not block command usage on
// partial refresh failures.
func autoRefreshConfiguredCaches(cfg *config.Config, c *cache.OrgCache) error {
	token := cfg.GetGitHubToken()
	if token == "" {
		return nil
	}

	client := github.NewClient(token)
	if err := client.ValidateToken(); err != nil {
		return fmt.Errorf("invalid GitHub token: %w", err)
	}

	var failures []string
	for _, org := range cfg.GetOrganizations() {
		if _, _, err := refreshReposIncrementally(
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
			failures = append(failures, fmt.Sprintf("%s: %v", org, err))
		}
	}

	if _, _, err := refreshReposIncrementally(
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
		failures = append(failures, fmt.Sprintf("%s: %v", cache.PersonalCacheKey, err))
	}

	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}

	return nil
}
