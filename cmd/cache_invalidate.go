package cmd

import (
	"fmt"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/spf13/cobra"
)

var invalidateCacheCmd = &cobra.Command{
	Use:   "invalidate [org]",
	Short: "Invalidate cache (all or specific organization)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runInvalidateCache,
}

func init() {
	cacheCmd.AddCommand(invalidateCacheCmd)
}

func runInvalidateCache(cmd *cobra.Command, args []string) error {
	c := cache.New()

	if len(args) == 1 {
		org := args[0]
		if err := c.Invalidate(org); err != nil {
			return fmt.Errorf("failed to invalidate cache for %s: %w", org, err)
		}
		fmt.Printf("✓ Cache invalidated for %s\n", org)
		return nil
	}

	orgs, err := c.ListAll()
	if err != nil {
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	if len(orgs) == 0 {
		fmt.Println("No cached organizations found")
		return nil
	}

	for _, org := range orgs {
		if err := c.Invalidate(org); err != nil {
			fmt.Printf("Failed to invalidate %s: %v\n", org, err)
			continue
		}
		fmt.Printf("✓ Cache invalidated for %s\n", org)
	}

	return nil
}
