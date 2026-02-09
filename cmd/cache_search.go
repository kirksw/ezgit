package cmd

import (
	"fmt"
	"strings"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/spf13/cobra"
)

var searchCacheCmd = &cobra.Command{
	Use:   "search <pattern>",
	Short: "Search cached repositories",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearchCache,
}

func init() {
	cacheCmd.AddCommand(searchCacheCmd)
}

func runSearchCache(cmd *cobra.Command, args []string) error {
	c := cache.New()

	pattern := args[0]
	repos, err := c.Search(pattern)
	if err != nil {
		return fmt.Errorf("failed to search cache: %w", err)
	}

	if len(repos) == 0 {
		fmt.Printf("No repositories found matching: %s\n", pattern)
		return nil
	}

	fmt.Printf("Found %d repositories matching '%s':\n\n", len(repos), pattern)

	for _, repo := range repos {
		fmt.Printf("  %s\n", repo.FullName)
		if repo.Description != "" {
			fmt.Printf("    %s\n", strings.TrimSpace(repo.Description))
		}
		fmt.Printf("    Stars: %d | Language: %s | Private: %v\n",
			repo.StargazersCount, repo.Language, repo.Private)
		fmt.Println()
	}

	return nil
}
