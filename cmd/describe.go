package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe <repo>",
	Short: "Describe local repository state as JSON",
	Args:  cobra.ExactArgs(1),
	RunE:  runDescribe,
}

type repoDescription struct {
	Host          string   `json:"host"`
	FullName      string   `json:"full_name"`
	DefaultBranch string   `json:"default_branch"`
	Path          string   `json:"path"`
	MetadataPath  string   `json:"metadata_path"`
	Cloned        bool     `json:"cloned"`
	Layout        string   `json:"layout"`
	Worktree      bool     `json:"worktree"`
	Worktrees     []string `json:"worktrees"`
}

func init() {
	rootCmd.AddCommand(describeCmd)
}

func runDescribe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	desc, err := describeRepo(cfg, args[0], git.New())
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(desc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode description: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func describeRepo(cfg *config.Config, repoInput string, lister repoWorktreeLister) (repoDescription, error) {
	repo, err := parseRepository(repoInput)
	if err != nil {
		return repoDescription{}, err
	}
	defaultBranch := ""
	if repo.Host == githubHost {
		defaultBranch = resolveDefaultBranch(repo.fullName(), "")
	}
	desc := repoDescription{Host: repo.Host, FullName: repo.fullName(), DefaultBranch: defaultBranch, Layout: "unknown"}
	repoPath := repo.repoPath(cfg)

	if abs, err := filepath.Abs(repoPath); err == nil {
		repoPath = abs
	}
	desc.Path = repoPath
	desc.MetadataPath = filepath.Join(repoPath, ".git")

	state, err := detectExistingRepoState(repoPath)
	if err != nil {
		return desc, err
	}
	desc.Layout = repoLayout(state)
	desc.Cloned = state == existingRepoRegular || state == existingRepoWorktree
	desc.Worktree = state == existingRepoWorktree
	if desc.Cloned && lister != nil {
		worktrees, err := lister.ListWorktrees(repoPath)
		if err != nil {
			return desc, fmt.Errorf("failed to list worktrees: %w", err)
		}
		desc.Worktrees = sortedStrings(worktrees)
	}
	return desc, nil
}

func repoLayout(state existingRepoState) string {
	switch state {
	case existingRepoMissing:
		return "missing"
	case existingRepoRegular:
		return "regular"
	case existingRepoWorktree:
		return "worktree"
	case existingRepoNonRepo:
		return "non_repo"
	default:
		return "unknown"
	}
}
