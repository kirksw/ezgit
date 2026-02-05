package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/ui"
	"github.com/kirksw/ezgit/internal/utils"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone [repo]",
	Short: "Clone a GitHub repository",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runClone,
}

var (
	worktree  bool
	branch    string
	depth     int
	quiet     bool
	keyPath   string
	cloneDest string
)

func init() {
	rootCmd.AddCommand(cloneCmd)

	cloneCmd.Flags().BoolVarP(&worktree, "worktree", "w", false, "clone as bare repo and create a worktree for the default branch")
	cloneCmd.Flags().StringVarP(&branch, "branch", "b", "", "clone specific branch")
	cloneCmd.Flags().IntVar(&depth, "depth", 0, "create a shallow clone with specified depth")
	cloneCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress output")
	cloneCmd.Flags().StringVar(&keyPath, "key-path", "", "SSH key path (default: ~/.ssh/id_rsa)")
	cloneCmd.Flags().StringVarP(&cloneDest, "dest", "d", "", "destination directory")
}

func runClone(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cmd.Flags().Changed("worktree") {
		worktree = cfg.Git.Worktree
	}

	if len(args) == 0 {
		return runFuzzyClone(cfg, cfg.Git.SeshOpen)
	}

	return runDirectClone(cfg, args[0], "")
}

func runFuzzyClone(cfg *config.Config, openMode bool) error {
	c := cache.New()
	allRepos, err := c.GetAllRepos()
	if err != nil {
		return fmt.Errorf("failed to load cached repos: %w", err)
	}

	if len(allRepos) == 0 {
		return fmt.Errorf("no cached repositories found. Run 'ezgit cache refresh' to fetch repos")
	}

	localRepos := utils.BuildLocalRepoMap(cfg.GetCloneDir(), allRepos)

	result, err := ui.RunFuzzySearch(allRepos, worktree, localRepos, openMode)
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}

	if result.Action == ui.ActionOpen {
		repoPath := getRepoPath(cfg, result.Repo.FullName, false, result.Repo.DefaultBranch)

		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			worktree = result.Worktree
			if err := runDirectClone(cfg, result.Repo.FullName, result.Repo.DefaultBranch); err != nil {
				return fmt.Errorf("failed to clone repo: %w", err)
			}
		} else {
			gitMgr := git.New()
			hasWorktrees, err := gitMgr.HasWorktrees(repoPath)
			if err != nil {
				return fmt.Errorf("failed to check for worktrees: %w", err)
			}
			if hasWorktrees && result.SelectedWorktree == "" {
				worktrees, err := gitMgr.ListWorktrees(repoPath)
				if err != nil {
					return fmt.Errorf("failed to list worktrees: %w", err)
				}

				var localOnlyRepos []github.Repo
				for _, repo := range allRepos {
					if localRepos[repo.FullName] {
						localOnlyRepos = append(localOnlyRepos, repo)
					}
				}

				result, err = ui.RunWorktreeSelection(localOnlyRepos, result.Repo, worktree, localRepos, worktrees)
				if err != nil {
					return err
				}

				if result.Repo == nil {
					return nil
				}
			}
		}

		return runSeshConnect(cfg, result.Repo.FullName, result.Repo.DefaultBranch, false, result.SelectedWorktree)
	}

	worktree = result.Worktree
	return runDirectClone(cfg, result.Repo.FullName, result.Repo.DefaultBranch)
}

func resolveDefaultBranch(repoInput, explicit string) string {
	if explicit != "" {
		return explicit
	}
	// Try to find the repo in cache to get its default branch.
	c := cache.New()
	allRepos, err := c.GetAllRepos()
	if err == nil {
		for _, r := range allRepos {
			if r.FullName == repoInput {
				return r.DefaultBranch
			}
		}
	}
	return "main"
}

func runDirectClone(cfg *config.Config, repoInput string, defaultBranch string) error {
	repoURL, err := git.ParseRepoURL(repoInput)
	if err != nil {
		return fmt.Errorf("invalid repo format: %w", err)
	}

	sshKey := keyPath
	if sshKey == "" {
		home, _ := os.UserHomeDir()
		sshKey = filepath.Join(home, ".ssh", "id_rsa")
	}

	gitMgr := git.New()

	if err := gitMgr.ValidateSSHKey(sshKey); err != nil {
		return fmt.Errorf("SSH key validation failed: %w", err)
	}

	dest := cloneDest
	if dest == "" {
		owner, repoName, err := config.ParseOwnerRepo(repoInput)
		if err != nil {
			parts := strings.Split(repoInput, "/")
			if len(parts) == 2 {
				owner = parts[0]
				repoName = parts[1]
			} else {
				repoName = filepath.Base(repoInput)
			}
		}

		cloneDir := cfg.GetCloneDir()
		if cloneDir != "" {
			if owner != "" {
				dest = filepath.Join(cloneDir, owner, repoName)
			} else {
				dest = filepath.Join(cloneDir, repoName)
			}
		} else {
			dest = filepath.Join(".", repoName)
		}
	}

	opts := git.CloneOptions{
		Bare:       worktree,
		Branch:     branch,
		Depth:      depth,
		Quiet:      quiet,
		SSHKeyPath: sshKey,
	}

	if !quiet {
		fmt.Printf("Cloning %s to %s\n", repoURL, dest)
	}

	if err := gitMgr.Clone(repoURL, dest, opts); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if worktree {
		branchName := resolveDefaultBranch(repoInput, defaultBranch)
		worktreePath := filepath.Join(dest, branchName)

		if !quiet {
			fmt.Printf("Creating worktree for branch '%s' at %s\n", branchName, worktreePath)
		}

		if err := gitMgr.CreateWorktree(dest, worktreePath, branchName); err != nil {
			return fmt.Errorf("failed to create worktree for branch %s: %w", branchName, err)
		}
	}

	if !quiet {
		fmt.Println("Clone successful!")
	}

	return nil
}

func runSeshConnect(cfg *config.Config, repoFullName, defaultBranch string, isWorktree bool, selectedWorktree string) error {
	cloneDir := cfg.GetCloneDir()
	if cloneDir == "" {
		return fmt.Errorf("clone_dir must be set in config to use 'ezgit open'")
	}

	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo format: %s", repoFullName)
	}

	owner, repoName := parts[0], parts[1]
	repoPath := filepath.Join(cloneDir, owner, repoName)

	if selectedWorktree != "" && selectedWorktree != "main" {
		repoPath = filepath.Join(repoPath, selectedWorktree)
	}

	cmd := exec.Command("sesh", "connect", repoPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
