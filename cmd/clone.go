package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/ui"
	"github.com/kirksw/ezgit/internal/utils"
	"github.com/spf13/cobra"
)

type existingRepoState int

const (
	existingRepoMissing existingRepoState = iota
	existingRepoRegular
	existingRepoWorktree
	existingRepoNonRepo
)

type existingRepoAction int

const (
	existingRepoActionCancel existingRepoAction = iota
	existingRepoActionOpen
	existingRepoActionConvert
)

type cloneCustomWorktree struct {
	Name       string
	BaseBranch string
}

type cloneWorktreePlan struct {
	CreateDefault bool
	CreateReview  bool
	Custom        []cloneCustomWorktree
}

var cloneCmd = &cobra.Command{
	Use:   "clone [repo] [worktreename]",
	Short: "Clone a GitHub repository",
	Args:  cobra.MaximumNArgs(2),
	RunE:  runClone,
}

var addCmd = &cobra.Command{
	Use:   "add <repo> <worktreename>",
	Short: "Add a worktree to a repository",
	Args:  cobra.ExactArgs(2),
	RunE:  runAdd,
}

var (
	worktree                bool
	branch                  string
	depth                   int
	quiet                   bool
	keyPath                 string
	cloneDest               string
	featureWorktree         string
	featureBaseBranch       string
	skipWorktreePrompt      bool // set internally to bypass interactive worktree prompts
	skipExistingPrompt      bool // set internally to bypass existing repo action prompt
	forcedClonePlan         *cloneWorktreePlan
	loadRepoDefaultBranches = loadRepoDefaultBranchesFromCache
	runZoxideAdd            = func(path string) error {
		cmd := exec.Command("zoxide", "add", path)
		_, err := cmd.CombinedOutput()
		return err
	}
)

var (
	defaultBranchLookupMu    sync.RWMutex
	defaultBranchLookupHome  string
	defaultBranchLookupRepos map[string]string
)

func init() {
	rootCmd.Flags().StringVarP(&branch, "branch", "b", "", "clone specific branch (non-worktree clones)")
	rootCmd.Flags().IntVar(&depth, "depth", 0, "create a shallow clone with specified depth")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress output")
	rootCmd.Flags().StringVar(&keyPath, "key-path", "", "SSH key path (default: ~/.ssh/id_rsa)")
	rootCmd.Flags().StringVarP(&cloneDest, "dest", "d", "", "destination directory")
	rootCmd.Flags().StringVar(&featureWorktree, "feature", "", "create an additional feature worktree (worktree layout only)")
	rootCmd.Flags().StringVar(&featureBaseBranch, "feature-base", "", "base branch for --feature (defaults to repository default branch)")
}

func runAdd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	return runCloneWithWorktreeAndBase(cfg, args[0], args[1], "")
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

	if worktree && branch != "" {
		return fmt.Errorf("--branch is not supported with --worktree; use --feature-base to control the feature worktree base branch")
	}

	if !worktree && (strings.TrimSpace(featureWorktree) != "" || strings.TrimSpace(featureBaseBranch) != "") {
		return fmt.Errorf("--feature and --feature-base require --worktree")
	}

	if len(args) == 0 {
		return runFuzzyClone(cfg, true)
	}

	if len(args) == 2 {
		worktreeName := args[1]
		return runCloneWithWorktree(cfg, args[0], worktreeName)
	}

	return runDirectClone(cfg, args[0], "", 0)
}

func runFuzzyClone(cfg *config.Config, openMode bool) error {
	c := cache.New()
	allRepos, err := c.GetAllRepos()
	if err != nil {
		return fmt.Errorf("failed to load cached repos: %w", err)
	}

	var backgroundRefreshDone <-chan error
	if len(allRepos) == 0 {
		if err := autoRefreshConfiguredCaches(cfg, c); err != nil {
			fmt.Printf("Warning: automatic cache refresh failed: %v\n", err)
		}

		allRepos, err = c.GetAllRepos()
		if err != nil {
			return fmt.Errorf("failed to load cached repos: %w", err)
		}
	} else {
		backgroundRefreshDone = startAutoRefreshConfiguredCaches(cfg, c)
		defer warnIfBackgroundAutoRefreshFailed(backgroundRefreshDone)
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
			originalSkipExistingPrompt := skipExistingPrompt
			skipExistingPrompt = true
			if err := runDirectClone(cfg, result.Repo.FullName, result.Repo.DefaultBranch, result.Repo.Size); err != nil {
				skipExistingPrompt = originalSkipExistingPrompt
				return fmt.Errorf("failed to clone repo: %w", err)
			}
			skipExistingPrompt = originalSkipExistingPrompt
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
	originalSkipExistingPrompt := skipExistingPrompt
	skipExistingPrompt = true
	defer func() {
		skipExistingPrompt = originalSkipExistingPrompt
	}()
	return runDirectClone(cfg, result.Repo.FullName, result.Repo.DefaultBranch, result.Repo.Size)
}

func resolveDefaultBranch(repoInput, explicit string) string {
	if explicit != "" {
		return explicit
	}

	homeDir, _ := os.UserHomeDir()

	defaultBranchLookupMu.RLock()
	if defaultBranchLookupRepos != nil && defaultBranchLookupHome == homeDir {
		if branchName := strings.TrimSpace(defaultBranchLookupRepos[repoInput]); branchName != "" {
			defaultBranchLookupMu.RUnlock()
			return branchName
		}
		defaultBranchLookupMu.RUnlock()
		return "main"
	}
	defaultBranchLookupMu.RUnlock()

	defaultBranchLookupMu.Lock()
	if defaultBranchLookupRepos == nil || defaultBranchLookupHome != homeDir {
		defaultBranchLookupRepos = loadRepoDefaultBranches()
		defaultBranchLookupHome = homeDir
	}
	branchName := strings.TrimSpace(defaultBranchLookupRepos[repoInput])
	defaultBranchLookupMu.Unlock()

	if branchName != "" {
		return branchName
	}
	return "main"
}

func loadRepoDefaultBranchesFromCache() map[string]string {
	defaultBranches := make(map[string]string)
	c := cache.New()
	allRepos, err := c.GetAllRepos()
	if err != nil {
		return defaultBranches
	}

	for _, repo := range allRepos {
		branchName := strings.TrimSpace(repo.DefaultBranch)
		if branchName == "" {
			continue
		}
		defaultBranches[repo.FullName] = branchName
	}

	return defaultBranches
}

func resetDefaultBranchLookupCache() {
	defaultBranchLookupMu.Lock()
	defer defaultBranchLookupMu.Unlock()
	defaultBranchLookupHome = ""
	defaultBranchLookupRepos = nil
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
	if !skipWorktreePrompt && !quiet && isInteractiveStdin() {
		cloneDepth = resolveCloneDepthForLargeRepo(cfg, repoInput, repoSizeHintKB, cloneDepth, os.Stdin, os.Stdout)
	}

	cloneTarget, metadataPath := resolveClonePaths(dest, worktree)
	repoState, err := detectExistingRepoState(dest)
	if err != nil {
		return err
	}

	didClone := true
	if repoState != existingRepoMissing {
		switch repoState {
		case existingRepoNonRepo:
			return fmt.Errorf("destination exists but is not a git repository: %s", dest)
		case existingRepoRegular:
			if !worktree {
				if !quiet {
					fmt.Printf("Repository already exists at %s; skipping clone\n", dest)
				}
				registerPathsWithZoxide([]string{dest}, quiet)
				return nil
			}

			if skipExistingPrompt {
				repoFullName, ok := extractRepoFullName(repoInput)
				if !ok {
					return fmt.Errorf("cannot open existing repository automatically for input %q; use 'ezgit open'", repoInput)
				}
				return runOpenCommand(cfg, repoFullName, "")
			}

			action, actionErr := resolveExistingRegularRepoAction(dest)
			if actionErr != nil {
				return actionErr
			}
			switch action {
			case existingRepoActionOpen:
				repoFullName, ok := extractRepoFullName(repoInput)
				if !ok {
					return fmt.Errorf("cannot open existing repository automatically for input %q; use 'ezgit open'", repoInput)
				}
				return runOpenCommand(cfg, repoFullName, "")
			case existingRepoActionConvert:
				convertDefaultBranch := resolveDefaultBranch(repoInput, defaultBranch)
				if err := runConvertPath(dest, convertDefaultBranch); err != nil {
					return err
				}
				registerRepoAndWorktreesWithZoxide(git.New(), dest, quiet)
				return nil
			default:
				return fmt.Errorf("cancelled")
			}
		case existingRepoWorktree:
			if !worktree {
				if !quiet {
					fmt.Printf("Repository already exists at %s in worktree layout; skipping clone\n", dest)
				}
				registerRepoAndWorktreesWithZoxide(gitMgr, dest, quiet)
				return nil
			}
			didClone = false
		}
	}

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

	if !quiet && didClone {
		fmt.Printf("Cloning %s to %s\n", repoURL, dest)
	}

	if didClone {
		if err := gitMgr.Clone(repoURL, cloneTarget, opts); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	} else if !quiet {
		fmt.Printf("Repository already exists at %s; skipping clone\n", dest)
	}

	if worktree {
		branchName := resolveDefaultBranch(repoInput, defaultBranch)
		branches := []string{branchName}
		if listedBranches, listErr := gitMgr.ListBranches(metadataPath); listErr == nil && len(listedBranches) > 0 {
			branches = listedBranches
		}

		if didClone {
			// Fix remote tracking: git clone --bare does not set a fetch refspec
			// and turns all remote branches into local branches.
			if err := gitMgr.ConfigureBareRemote(metadataPath, branchName); err != nil {
				return fmt.Errorf("failed to configure bare remote: %w", err)
			}
		}
		interactive := !skipWorktreePrompt && !quiet && isInteractiveStdin()
		createDefaultWorktree := true
		createReviewWorktree := true
		customWorktrees := make([]cloneCustomWorktree, 0)

		if forcedClonePlan != nil {
			createDefaultWorktree = forcedClonePlan.CreateDefault
			createReviewWorktree = forcedClonePlan.CreateReview
			customWorktrees = append(customWorktrees, forcedClonePlan.Custom...)
		} else if interactive {
			plan, cancelled, planErr := ui.RunCloneWorktreePlanPrompt(branches, branchName, true)
			if planErr != nil {
				return fmt.Errorf("failed to configure clone worktree options: %w", planErr)
			}
			if cancelled {
				return fmt.Errorf("cancelled")
			}
			createDefaultWorktree = plan.CreateDefault
			createReviewWorktree = plan.CreateReview
			for _, custom := range plan.Custom {
				customWorktrees = append(customWorktrees, cloneCustomWorktree{
					Name:       custom.Name,
					BaseBranch: custom.BaseBranch,
				})
			}
		}

		featureBranch := ""
		baseBranch := ""
		if strings.TrimSpace(featureWorktree) != "" || strings.TrimSpace(featureBaseBranch) != "" {
			featureBranch, baseBranch, err = resolveFeatureWorktreeConfig(branchName, featureWorktree, featureBaseBranch)
			if err != nil {
				return err
			}
		}
		if featureBranch != "" {
			customWorktrees = append(customWorktrees, cloneCustomWorktree{
				Name:       featureBranch,
				BaseBranch: baseBranch,
			})
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

		for _, custom := range customWorktrees {
			customName := strings.TrimSpace(custom.Name)
			if customName == "" {
				continue
			}
			customBase := strings.TrimSpace(custom.BaseBranch)
			if customBase == "" {
				customBase = branchName
			}
			featureWorktreePath := filepath.Join(dest, customName)
			if !quiet {
				fmt.Printf("Creating feature worktree %q from %q\n", customName, customBase)
			}
			if err := gitMgr.CreateFeatureWorktree(metadataPath, featureWorktreePath, customName, customBase); err != nil {
				return fmt.Errorf("failed to create feature worktree %s from %s: %w", customName, customBase, err)
			}
			if !quiet {
				fmt.Printf("Worktree created: %s\n", featureWorktreePath)
			}
		}
	}

	if !quiet {
		fmt.Println("Clone successful!")
	}

	if worktree {
		registerRepoAndWorktreesWithZoxide(gitMgr, dest, quiet)
	} else {
		registerPathsWithZoxide([]string{dest}, quiet)
	}

	return nil
}

// runCloneWithWorktree handles `ezgit clone <repo> <worktreename>`.
// If the repo is already cloned, it just adds the worktree without prompts.
// If not yet cloned, it clones with default worktrees (default branch + review)
// plus the specified worktree, all without prompts.
func runCloneWithWorktree(cfg *config.Config, repoInput string, worktreeName string) error {
	return runCloneWithWorktreeAndBase(cfg, repoInput, worktreeName, "")
}

func runCloneWithWorktreeAndBase(cfg *config.Config, repoInput string, worktreeName string, baseBranch string) error {
	worktreeName = strings.TrimSpace(worktreeName)
	if worktreeName == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}
	baseBranch = strings.TrimSpace(baseBranch)

	// Force worktree mode.
	originalWorktree := worktree
	worktree = true
	defer func() {
		worktree = originalWorktree
	}()

	dest, metadataPath, err := resolveRepoPaths(cfg, repoInput)
	if err != nil {
		return err
	}

	gitMgr := git.New()

	// Check if the repo is already cloned.
	repoState, stateErr := detectExistingRepoState(dest)
	if stateErr != nil {
		return stateErr
	}
	if repoState != existingRepoMissing {
		switch repoState {
		case existingRepoWorktree:
			return addWorktreeToRepo(gitMgr, repoInput, dest, metadataPath, worktreeName)
		case existingRepoRegular:
			action, actionErr := resolveExistingRegularRepoAction(dest)
			if actionErr != nil {
				return actionErr
			}
			switch action {
			case existingRepoActionOpen:
				repoFullName, ok := extractRepoFullName(repoInput)
				if !ok {
					return fmt.Errorf("cannot open existing repository automatically for input %q; use 'ezgit open'", repoInput)
				}
				return runOpenCommand(cfg, repoFullName, "")
			case existingRepoActionConvert:
				convertDefaultBranch := resolveDefaultBranch(repoInput, "")
				if err := runConvertPath(dest, convertDefaultBranch); err != nil {
					return err
				}
				registerRepoAndWorktreesWithZoxide(gitMgr, dest, quiet)
				return addWorktreeToRepo(gitMgr, repoInput, dest, metadataPath, worktreeName)
			default:
				return fmt.Errorf("cancelled")
			}
		default:
			return fmt.Errorf("destination exists but is not a git repository: %s", dest)
		}
	}

	// Not cloned yet — perform full clone with defaults + the specified worktree, no prompts.
	originalFeatureWorktree := featureWorktree
	originalFeatureBaseBranch := featureBaseBranch
	originalSkipPrompt := skipWorktreePrompt
	featureWorktree = worktreeName
	featureBaseBranch = baseBranch
	skipWorktreePrompt = true
	defer func() {
		featureWorktree = originalFeatureWorktree
		featureBaseBranch = originalFeatureBaseBranch
		skipWorktreePrompt = originalSkipPrompt
	}()

	if err := runDirectClone(cfg, repoInput, "", 0); err != nil {
		return err
	}

	return nil
}

// addWorktreeToRepo creates a feature worktree in an existing bare repo.
func addWorktreeToRepo(gitMgr git.GitManager, repoInput string, dest string, metadataPath string, worktreeName string) error {
	worktreeName = strings.TrimSpace(worktreeName)
	if worktreeName == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	branchName := resolveDefaultBranch(repoInput, "")
	featureBranch, baseBranch, err := resolveFeatureWorktreeConfig(branchName, worktreeName, "")
	if err != nil {
		return err
	}

	if featureBranch == "" {
		return nil
	}

	worktreePath := filepath.Join(dest, worktreeName)

	if !quiet {
		fmt.Printf("Creating feature worktree %q from %q\n", worktreeName, branchName)
	}
	if err := gitMgr.CreateFeatureWorktree(metadataPath, worktreePath, featureBranch, baseBranch); err != nil {
		return fmt.Errorf("failed to create worktree %s: %w", worktreeName, err)
	}
	if !quiet {
		fmt.Printf("Worktree created: %s\n", worktreePath)
	}
	registerPathsWithZoxide([]string{dest, worktreePath}, quiet)
	return nil
}

func detectExistingRepoState(dest string) (existingRepoState, error) {
	info, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return existingRepoMissing, nil
	}
	if err != nil {
		return existingRepoMissing, fmt.Errorf("failed to inspect destination %s: %w", dest, err)
	}
	if !info.IsDir() {
		return existingRepoNonRepo, nil
	}

	gitMetadataPath := filepath.Join(dest, ".git")
	if _, err := os.Stat(gitMetadataPath); err == nil {
		isBare, bareErr := isBareGitDir(gitMetadataPath)
		if bareErr != nil {
			return existingRepoNonRepo, nil
		}
		if isBare {
			return existingRepoWorktree, nil
		}
		return existingRepoRegular, nil
	}

	if _, err := os.Stat(filepath.Join(dest, "HEAD")); err == nil {
		if _, err := os.Stat(filepath.Join(dest, "config")); err == nil {
			isBare, bareErr := isBareGitDir(dest)
			if bareErr == nil && isBare {
				return existingRepoWorktree, nil
			}
		}
	}

	return existingRepoNonRepo, nil
}

func isBareGitDir(gitDir string) (bool, error) {
	cmd := exec.Command("git", "--git-dir", gitDir, "rev-parse", "--is-bare-repository")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to detect git dir type: %w", err)
	}
	value := strings.TrimSpace(string(output))
	return value == "true", nil
}

func resolveExistingRegularRepoAction(dest string) (existingRepoAction, error) {
	if !isInteractiveStdin() {
		return existingRepoActionCancel, fmt.Errorf("destination %s is an existing non-worktree clone; re-run interactively to choose open or auto-convert", dest)
	}

	for {
		fmt.Printf("Repository already exists at %s and is not in worktree layout. Choose action: [o]pen existing, [c]onvert to worktree, [x] cancel: ", dest)
		input, err := readLineTrimmed(os.Stdin)
		if err != nil {
			return existingRepoActionCancel, fmt.Errorf("failed to read action: %w", err)
		}
		switch strings.ToLower(input) {
		case "o", "open":
			return existingRepoActionOpen, nil
		case "c", "convert":
			return existingRepoActionConvert, nil
		case "x", "cancel":
			return existingRepoActionCancel, nil
		default:
			fmt.Println("Please choose one of: o, c, x")
		}
	}
}

func readLineTrimmed(in io.Reader) (string, error) {
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func registerRepoAndWorktreesWithZoxide(gitMgr git.GitManager, repoRoot string, quiet bool) {
	paths := []string{repoRoot}
	worktrees, err := gitMgr.ListWorktrees(repoRoot)
	if err == nil {
		for _, wt := range worktrees {
			wt = strings.TrimSpace(wt)
			if wt == "" {
				continue
			}
			paths = append(paths, filepath.Join(repoRoot, wt))
		}
	}
	registerPathsWithZoxide(paths, quiet)
}

func registerPathsWithZoxide(paths []string, quiet bool) {
	seen := make(map[string]struct{})
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		absPath, err := filepath.Abs(trimmed)
		if err != nil {
			continue
		}
		if _, ok := seen[absPath]; ok {
			continue
		}
		seen[absPath] = struct{}{}

		if err := runZoxideAdd(absPath); err != nil && !quiet {
			fmt.Printf("Warning: failed to add %s to zoxide: %v\n", absPath, err)
		}
	}
}

// resolveRepoPaths returns the repo destination directory and the bare metadata path.
func resolveRepoPaths(cfg *config.Config, repoInput string) (dest string, metadataPath string, err error) {
	if cloneDest != "" {
		dest = cloneDest
	} else {
		owner, repoName, parseErr := config.ParseOwnerRepo(repoInput)
		if parseErr != nil {
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

	metadataPath = filepath.Join(dest, ".git")
	return dest, metadataPath, nil
}

func resolveOpenTargetPath(repoPath, selectedWorktree string) string {
	selectedWorktree = strings.TrimSpace(selectedWorktree)
	if selectedWorktree == "" {
		return repoPath
	}
	return filepath.Join(repoPath, selectedWorktree)
}
