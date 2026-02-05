package cmd

import (
	"fmt"
	"time"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage repository cache",
}

var (
	forceRefresh bool
	ttlString    string
)

func init() {
	rootCmd.AddCommand(cacheCmd)

	cacheCmd.PersistentFlags().BoolVar(&forceRefresh, "force", false, "force refresh even if not expired")
	cacheCmd.PersistentFlags().StringVar(&ttlString, "ttl", "", "set custom TTL (e.g., 24h, 1h30m)")
}

func loadConfigAndCache() (*config.Config, *cache.OrgCache, string, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to load config: %w", err)
	}

	token := cfg.GetGitHubToken()
	if token == "" {
		return nil, nil, "", fmt.Errorf("GITHUB_TOKEN not set in config or environment")
	}

	c := cache.New()

	if ttlString != "" {
		duration, err := time.ParseDuration(ttlString)
		if err != nil {
			return nil, nil, "", fmt.Errorf("invalid TTL format: %w", err)
		}
		c.SetTTL(duration)
	}

	return cfg, c, token, nil
}
