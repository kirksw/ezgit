package cmd

import (
	"fmt"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/ui"
	"github.com/kirksw/ezgit/internal/utils"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch unified interactive clone/open/connect TUI",
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	c := cache.New()
	if err := autoRefreshConfiguredCaches(cfg, c); err != nil {
		fmt.Printf("Warning: automatic cache refresh failed: %v\n", err)
	}

	allRepos, err := c.GetAllRepos()
	if err != nil {
		fmt.Printf("Warning: failed to load cached repos: %v\n", err)
		allRepos = []github.Repo{}
	}

	cloneDir := cfg.GetCloneDir()
	localRepos := make(map[string]bool)
	localOnly := make([]github.Repo, 0, len(allRepos))
	if cloneDir != "" {
		localRepos = utils.BuildLocalRepoMap(cloneDir, allRepos)
		for _, repo := range allRepos {
			if localRepos[repo.FullName] {
				localOnly = append(localOnly, repo)
			}
		}
	}

	sessions, sessionErr := listTmuxSessions()
	if sessionErr != nil {
		sessions = []string{}
	}

	result, err := ui.RunHub(allRepos, localOnly, localRepos, sessions, cfg.Git.Worktree)
	if err != nil {
		return err
	}
	if result == nil || result.Cancelled {
		return nil
	}

	switch result.Mode {
	case ui.HubModeClone:
		if result.Repo == nil {
			return nil
		}
		worktree = result.Worktree
		return runDirectClone(cfg, result.Repo.FullName, result.Repo.DefaultBranch, result.Repo.Size)
	case ui.HubModeOpen:
		if result.Repo == nil {
			return nil
		}
		if result.Convert {
			repoPath := getRepoPath(cfg, result.Repo.FullName, false, result.Repo.DefaultBranch)
			return runConvertPath(repoPath, result.Repo.DefaultBranch)
		}
		return runOpenRepoSelection(cfg, result.Repo, localOnly, localRepos, "")
	case ui.HubModeConnect:
		if result.Session == "" {
			if sessionErr != nil {
				return sessionErr
			}
			return fmt.Errorf("no tmux sessions found")
		}
		return attachTmuxSession(result.Session)
	default:
		return nil
	}
}
