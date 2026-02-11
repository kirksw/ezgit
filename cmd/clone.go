package cmd

import (
	"fmt"
	"os"
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
	worktree          bool
	branch            string
	depth             int
	quiet             bool
	keyPath           string
	cloneDest         string
	featureWorktree   string
	featureBaseBranch string
)

func init() {
	rootCmd.AddCommand(cloneCmd)

	cloneCmd.Flags().BoolVarP(&worktree, "worktree", "w", false, "clone metadata into .git and create worktree(s)")
	cloneCmd.Flags().StringVarP(&branch, "branch", "b", "", "clone specific branch")
	cloneCmd.Flags().IntVar(&depth, "depth", 0, "create a shallow clone with specified depth")
	cloneCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress output")
	cloneCmd.Flags().StringVar(&keyPath, "key-path", "", "SSH key path (default: ~/.ssh/id_rsa)")
	cloneCmd.Flags().StringVarP(&cloneDest, "dest", "d", "", "destination directory")
	cloneCmd.Flags().StringVar(&featureWorktree, "feature", "", "create an additional feature worktree using this branch name (worktree mode)")
	cloneCmd.Flags().StringVar(&featureBaseBranch, "feature-base", "", "base branch for --feature (defaults to repository default branch)")
}

func resolveClonePaths(dest string, asWorktree bool) (cloneTarget string, metadataPath string) {
	if !asWorktree {
		return dest, dest
	}
	metadataPath = filepath.Join(dest, ".git")
	return metadataPath, metadataPath
}

func resolveFeatureWorktreeConfig(defaultBranch, featureBranchInput, baseBranchInput string) (featureBranch string, baseBranch string, err error) {
	featureBranch = strings.TrimSpace(featureBranchInput)
	baseBranch = strings.TrimSpace(baseBranchInput)

	if featureBranch == "" {
		if baseBranch != "" {
			return "", "", fmt.Errorf("--feature-base requires --feature")
		}
		return "", "", nil
	}

	if baseBranch == "" {
		baseBranch = defaultBranch
	}

	if featureBranch == "review" {
		return "", "", fmt.Errorf("--feature value %q is reserved", featureBranch)
	}

	if featureBranch == defaultBranch {
		return "", "", fmt.Errorf("--feature value %q conflicts with the default branch worktree", featureBranch)
	}

	return featureBranch, baseBranch, nil
}

func runClone(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cmd.Flags().Changed("worktree") {
		worktree = cfg.Git.Worktree
	}

	if worktree && branch != "" {
		return fmt.Errorf("--branch is not supported with --worktree; use --feature-base to control the feature worktree base branch")
	}

	if !worktree && (strings.TrimSpace(featureWorktree) != "" || strings.TrimSpace(featureBaseBranch) != "") {
		return fmt.Errorf("--feature and --feature-base require --worktree")
	}

	if len(args) == 0 {
		return runFuzzyClone(cfg, cfg.Git.SeshOpen)
	}

	return runDirectClone(cfg, args[0], "", 0)
}

func runFuzzyClone(cfg *config.Config, openMode bool) error {
	c := cache.New()
	if err := autoRefreshConfiguredCaches(cfg, c); err != nil {
		fmt.Printf("Warning: automatic cache refresh failed: %v\n", err)
	}

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
			if err := runDirectClone(cfg, result.Repo.FullName, result.Repo.DefaultBranch, result.Repo.Size); err != nil {
				return fmt.Errorf("failed to clone repo: %w", err)
			}
			localRepos[result.Repo.FullName] = true
		}

		var localOnlyRepos []github.Repo
		for _, repo := range allRepos {
			if localRepos[repo.FullName] {
				localOnlyRepos = append(localOnlyRepos, repo)
			}
		}

		return runOpenRepoSelection(cfg, result.Repo, localOnlyRepos, localRepos, result.SelectedWorktree)
	}

	worktree = result.Worktree
	return runDirectClone(cfg, result.Repo.FullName, result.Repo.DefaultBranch, result.Repo.Size)
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

func runDirectClone(cfg *config.Config, repoInput string, defaultBranch string, repoSizeHintKB int) error {
	if worktree && branch != "" {
		return fmt.Errorf("--branch is not supported with --worktree; use --feature-base to control the feature worktree base branch")
	}
	if !worktree && (strings.TrimSpace(featureWorktree) != "" || strings.TrimSpace(featureBaseBranch) != "") {
		return fmt.Errorf("--feature and --feature-base require --worktree")
	}

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

	cloneDepth := depth
	if !quiet && isInteractiveStdin() {
		cloneDepth = resolveCloneDepthForLargeRepo(cfg, repoInput, repoSizeHintKB, cloneDepth, os.Stdin, os.Stdout)
	}

	cloneTarget, metadataPath := resolveClonePaths(dest, worktree)
	if worktree {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	opts := git.CloneOptions{
		Bare:       worktree,
		Branch:     branch,
		Depth:      cloneDepth,
		Quiet:      quiet,
		SSHKeyPath: sshKey,
	}

	if !quiet {
		fmt.Printf("Cloning %s to %s\n", repoURL, dest)
	}

	if err := gitMgr.Clone(repoURL, cloneTarget, opts); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if worktree {
		branchName := resolveDefaultBranch(repoInput, defaultBranch)

		// Fix remote tracking: git clone --bare does not set a fetch refspec
		// and turns all remote branches into local branches.
		if err := gitMgr.ConfigureBareRemote(metadataPath, branchName); err != nil {
			return fmt.Errorf("failed to configure bare remote: %w", err)
		}
		interactive := !quiet && isInteractiveStdin()
		createDefaultWorktree := true
		createReviewWorktree := true
		requestCustomWorktree := false

		if interactive {
			var cancelled bool
			createDefaultWorktree, createReviewWorktree, requestCustomWorktree, cancelled, err = ui.RunCloneWorktreeOptionsPrompt(branchName)
			if err != nil {
				return fmt.Errorf("failed to configure clone worktree options: %w", err)
			}
			if cancelled {
				return fmt.Errorf("cancelled")
			}
		}

		branches := []string{branchName}
		if listedBranches, listErr := gitMgr.ListBranches(metadataPath); listErr == nil && len(listedBranches) > 0 {
			branches = listedBranches
		}

		featureBranch := ""
		baseBranch := ""
		if strings.TrimSpace(featureWorktree) != "" || strings.TrimSpace(featureBaseBranch) != "" {
			featureBranch, baseBranch, err = resolveFeatureWorktreeConfig(branchName, featureWorktree, featureBaseBranch)
			if err != nil {
				return err
			}
		} else if interactive && requestCustomWorktree {
			var cancelled bool
			featureBranch, baseBranch, cancelled, err = ui.RunCreateWorktreePrompt(branches, branchName)
			if err != nil {
				return fmt.Errorf("failed to configure feature worktree: %w", err)
			}
			if cancelled {
				return fmt.Errorf("cancelled")
			}
			featureBranch, baseBranch, err = resolveFeatureWorktreeConfig(branchName, featureBranch, baseBranch)
			if err != nil {
				return err
			}
		}

		if createDefaultWorktree {
			if !quiet {
				fmt.Printf("Creating default worktree for %q\n", branchName)
			}
			defaultWorktreePath := filepath.Join(dest, branchName)
			if err := gitMgr.CreateWorktree(metadataPath, defaultWorktreePath, branchName); err != nil {
				return fmt.Errorf("failed to create default worktree for branch %s: %w", branchName, err)
			}
			if !quiet {
				fmt.Printf("Worktree created: %s\n", defaultWorktreePath)
			}
		}

		if createReviewWorktree {
			reviewWorktreePath := filepath.Join(dest, "review")
			if !quiet {
				fmt.Printf("Creating review worktree from %q\n", branchName)
			}
			if err := gitMgr.CreateDetachedWorktree(metadataPath, reviewWorktreePath, branchName); err != nil {
				return fmt.Errorf("failed to create review worktree from branch %s: %w", branchName, err)
			}
			if !quiet {
				fmt.Printf("Worktree created: %s\n", reviewWorktreePath)
			}
		}

		if featureBranch != "" {
			featureWorktreePath := filepath.Join(dest, featureBranch)
			if !quiet {
				fmt.Printf("Creating feature worktree %q from %q\n", featureBranch, baseBranch)
			}
			if err := gitMgr.CreateFeatureWorktree(metadataPath, featureWorktreePath, featureBranch, baseBranch); err != nil {
				return fmt.Errorf("failed to create feature worktree %s from %s: %w", featureBranch, baseBranch, err)
			}
			if !quiet {
				fmt.Printf("Worktree created: %s\n", featureWorktreePath)
			}
		}
	}

	if !quiet {
		fmt.Println("Clone successful!")
	}

	return nil
}

func resolveOpenTargetPath(repoPath, selectedWorktree string) string {
	selectedWorktree = strings.TrimSpace(selectedWorktree)
	if selectedWorktree == "" {
		return repoPath
	}
	return filepath.Join(repoPath, selectedWorktree)
}
