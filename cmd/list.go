package cmd

import (
	"fmt"
	"sort"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/utils"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List cached GitHub data",
}

var listOrgsCmd = &cobra.Command{
	Use:   "orgs",
	Short: "List cached organizations",
	Args:  cobra.NoArgs,
	RunE:  runListOrgs,
}

var listReposCmd = &cobra.Command{
	Use:   "repos",
	Short: "List cached repositories",
	Args:  cobra.NoArgs,
	RunE:  runListRepos,
}

var listWorktreesCmd = &cobra.Command{
	Use:   "worktrees <repo>",
	Short: "List local worktrees for a repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runListWorktrees,
}

var listReposLocalOnly bool

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.AddCommand(listOrgsCmd, listReposCmd, listWorktreesCmd)
	listReposCmd.Flags().BoolVar(&listReposLocalOnly, "local", false, "only list repos already cloned under clone_dir")
}

func runListOrgs(cmd *cobra.Command, args []string) error {
	orgs, err := cache.New().ListAll()
	if err != nil {
		return fmt.Errorf("failed to list cached organizations: %w", err)
	}
	for _, org := range orgs {
		fmt.Println(org)
	}
	return nil
}

func runListRepos(cmd *cobra.Command, args []string) error {
	repos, err := collectCachedRepos(cache.New())
	if err != nil {
		return err
	}

	localRepos := map[string]bool(nil)
	if listReposLocalOnly {
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		localRepos = utils.BuildLocalRepoMap(cfg.GetCloneDir(), repos)
	}

	for _, name := range sortedRepoNames(repos, localRepos, listReposLocalOnly) {
		fmt.Println(name)
	}
	return nil
}

func collectCachedRepos(c *cache.OrgCache) ([]github.Repo, error) {
	orgs, err := c.ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list cached organizations: %w", err)
	}

	seen := make(map[string]struct{})
	var repos []github.Repo
	for _, org := range orgs {
		cached, err := c.GetStale(org)
		if err != nil {
			continue
		}
		for _, repo := range cached.Repos {
			if repo.FullName == "" {
				continue
			}
			if _, ok := seen[repo.FullName]; ok {
				continue
			}
			seen[repo.FullName] = struct{}{}
			repos = append(repos, repo)
		}
	}
	return repos, nil
}

func runListWorktrees(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	repoFullName, ok := extractRepoFullName(args[0])
	if !ok {
		return fmt.Errorf("invalid repo format: %s", args[0])
	}

	repoPath := getRepoPath(cfg, repoFullName, false, resolveDefaultBranch(repoFullName, ""))
	state, err := detectExistingRepoState(repoPath)
	if err != nil {
		return err
	}
	if state == existingRepoMissing || state == existingRepoRegular {
		return nil
	}
	if state == existingRepoNonRepo {
		return fmt.Errorf("destination exists but is not a git repository: %s", repoPath)
	}

	worktrees, err := git.New().ListWorktrees(repoPath)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}
	for _, worktree := range sortedStrings(worktrees) {
		fmt.Println(worktree)
	}
	return nil
}

func sortedRepoNames(repos []github.Repo, localRepos map[string]bool, localOnly bool) []string {
	names := make([]string, 0, len(repos))
	for _, repo := range repos {
		if repo.FullName == "" || (localOnly && !localRepos[repo.FullName]) {
			continue
		}
		names = append(names, repo.FullName)
	}
	return sortedStrings(names)
}

func sortedStrings(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
}
