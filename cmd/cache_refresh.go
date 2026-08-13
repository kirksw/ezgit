package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/spf13/cobra"
)

var refreshCacheCmd = &cobra.Command{
	Use:   "refresh [org]",
	Short: "Refresh cache for organization(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRefreshCache,
}

func init() {
	cacheCmd.AddCommand(refreshCacheCmd)
}

func runRefreshCache(cmd *cobra.Command, args []string) error {
	cfg, c, token, err := loadConfigAndCache()
	if err != nil {
		return err
	}

	client := github.NewClient(token)

	if err := client.ValidateToken(); err != nil {
		return fmt.Errorf("invalid GitHub token: %w", err)
	}

	refreshPersonal := len(args) == 0
	orgs := cfg.GetOrganizations()

	if len(args) == 1 {
		if args[0] == cache.PersonalCacheKey {
			refreshPersonal = true
			orgs = nil
		} else {
			orgs = []string{args[0]}
		}
	} else if len(orgs) == 0 && !refreshPersonal {
		return fmt.Errorf("no organizations specified in config or as argument")
	}

	for _, org := range orgs {
		fmt.Printf("Refreshing cache for %s...\n", org)

		added, total, err := refreshReposIncrementally(
			c,
			org,
			forceRefresh,
			func() ([]github.Repo, error) {
				return client.FetchOrgRepos(org)
			},
			func(createdAfter time.Time) ([]github.Repo, error) {
				return client.FetchOrgReposCreatedAfter(org, createdAfter)
			},
		)

		if err != nil {
			fmt.Printf("Failed to refresh %s: %v\n", org, err)
			continue
		}

		fmt.Printf("✓ Cached %d repositories from %s", total, org)
		if !forceRefresh {
			if added > 0 {
				fmt.Printf(" (+%d new)", added)
			} else {
				fmt.Printf(" (no new repos)")
			}
		}
		fmt.Println()
	}

	if refreshPersonal {
		fmt.Printf("Refreshing personal repos...\n")

		added, total, err := refreshReposIncrementally(
			c,
			cache.PersonalCacheKey,
			forceRefresh,
			func() ([]github.Repo, error) {
				return client.FetchPrivateRepos()
			},
			func(createdAfter time.Time) ([]github.Repo, error) {
				return client.FetchPrivateReposCreatedAfter(createdAfter)
			},
		)

		if err != nil {
			fmt.Printf("Failed to refresh personal repos: %v\n", err)
		} else {
			fmt.Printf("✓ Cached %d personal repositories", total)
			if !forceRefresh {
				if added > 0 {
					fmt.Printf(" (+%d new)", added)
				} else {
					fmt.Printf(" (no new repos)")
				}
			}
			fmt.Println()
		}
	}

	return nil
}

func refreshReposIncrementally(
	c *cache.OrgCache,
	cacheKey string,
	fullRefresh bool,
	fetchAll func() ([]github.Repo, error),
	fetchCreatedAfter func(time.Time) ([]github.Repo, error),
) (added int, total int, err error) {
	if fullRefresh {
		repos, err := fetchAll()
		if err != nil {
			return 0, 0, err
		}
		if err := c.Set(cacheKey, repos); err != nil {
			return 0, 0, err
		}
		return len(repos), len(repos), nil
	}

	latestCreatedAt, err := c.GetLatestRepoCreatedAt(cacheKey)
	if err != nil {
		repos, err := fetchAll()
		if err != nil {
			return 0, 0, err
		}
		if err := c.Set(cacheKey, repos); err != nil {
			return 0, 0, err
		}
		return len(repos), len(repos), nil
	}

	existing, err := c.GetStale(cacheKey)
	if err != nil {
		repos, err := fetchAll()
		if err != nil {
			return 0, 0, err
		}
		if err := c.Set(cacheKey, repos); err != nil {
			return 0, 0, err
		}
		return len(repos), len(repos), nil
	}

	newRepos, err := fetchCreatedAfter(latestCreatedAt)
	if err != nil {
		return 0, 0, err
	}

	merged, added := mergeReposByFullName(existing.Repos, newRepos)
	if err := c.Set(cacheKey, merged); err != nil {
		return 0, 0, err
	}

	return added, len(merged), nil
}

func mergeReposByFullName(existing, incoming []github.Repo) ([]github.Repo, int) {
	repoByName := make(map[string]github.Repo, len(existing)+len(incoming))
	for _, repo := range existing {
		repoByName[repo.FullName] = repo
	}

	added := 0
	for _, repo := range incoming {
		if _, ok := repoByName[repo.FullName]; !ok {
			added++
		}
		repoByName[repo.FullName] = repo
	}

	merged := make([]github.Repo, 0, len(repoByName))
	for _, repo := range repoByName {
		merged = append(merged, repo)
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].CreatedAt.After(merged[j].CreatedAt)
	})

	return merged, added
}
