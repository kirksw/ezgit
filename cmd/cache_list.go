package cmd

import (
	"fmt"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/spf13/cobra"
)

var listCacheCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cached organizations",
	RunE:  runListCache,
}

func init() {
	cacheCmd.AddCommand(listCacheCmd)
}

func runListCache(cmd *cobra.Command, args []string) error {
	c := cache.New()

	orgs, err := c.ListAll()
	if err != nil {
		return fmt.Errorf("failed to list cached organizations: %w", err)
	}

	if len(orgs) == 0 {
		fmt.Println("No cached organizations found")
		return nil
	}

	fmt.Println("Cached organizations:")
	for _, org := range orgs {
		cached, err := c.Get(org)
		if err != nil {
			fmt.Printf("  %s (error loading cache)\n", org)
			continue
		}
		fmt.Printf("  %s (%d repos, cached: %s)\n", org, len(cached.Repos), cached.CachedAt.Format("2006-01-02 15:04"))
	}

	return nil
}
