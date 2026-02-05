package cmd

import (
	"fmt"

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
		if !forceRefresh && !c.IsExpired(org) {
			fmt.Printf("Cache for %s is still valid (use --force to refresh)\n", org)
			continue
		}

		fmt.Printf("Refreshing cache for %s...\n", org)

		err := c.Refresh(org, func() ([]github.Repo, error) {
			return client.FetchOrgRepos(org)
		})

		if err != nil {
			fmt.Printf("Failed to refresh %s: %v\n", org, err)
			continue
		}

		cached, err := c.Get(org)
		if err == nil {
			fmt.Printf("✓ Cached %d repositories from %s\n", len(cached.Repos), org)
		}
	}

	if refreshPersonal {
		if !forceRefresh && !c.IsExpired(cache.PersonalCacheKey) {
			fmt.Printf("Personal cache is still valid (use --force to refresh)\n")
		} else {
			fmt.Printf("Refreshing personal repos...\n")

			err := c.Refresh(cache.PersonalCacheKey, func() ([]github.Repo, error) {
				return client.FetchPrivateRepos()
			})

			if err != nil {
				fmt.Printf("Failed to refresh personal repos: %v\n", err)
			} else {
				cached, err := c.Get(cache.PersonalCacheKey)
				if err == nil {
					fmt.Printf("✓ Cached %d personal repositories\n", len(cached.Repos))
				}
			}
		}
	}

	return nil
}
