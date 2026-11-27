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

func defaultCLICloneWorktreePlan() cloneWorktreePlan {
	return cloneWorktreePlan{CreateDefault: true}
}

func cliCloneWorktreePlanFor(defaultBranch, worktreeName string) cloneWorktreePlan {
	plan := defaultCLICloneWorktreePlan()
	switch {
	case worktreeName == "review":
		plan.CreateReview = true
	case worktreeName != defaultBranch:
		plan.Custom = []cloneCustomWorktree{{Name: worktreeName, BaseBranch: defaultBranch}}
	}
	return plan
}

var cloneCmd = &cobra.Command{
	Use:   "clone <repo>",
	Short: "Clone a GitHub repository",
	Args:  cobra.RangeArgs(1, 2),
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
	rootCmd.AddCommand(cloneCmd, addCmd, openCmd)
	addCmd.Hidden = true
	addCmd.Deprecated = "use ezgit wt add instead"

	addCloneFlags(rootCmd, false)
	addCloneFlags(cloneCmd, true)
	for _, name := range []string{"worktree", "bare", "feature", "feature-base", "dest"} {
		_ = cloneCmd.Flags().MarkDeprecated(name, "use the managed clone layout and ezgit wt add")
	}
	for _, name := range []string{"feature", "feature-base", "dest"} {
		_ = rootCmd.Flags().MarkDeprecated(name, "use the managed clone layout and ezgit wt add")
	}
}

func addCloneFlags(cmd *cobra.Command, includeLayout bool) {
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "clone specific branch (non-worktree clones)")
	cmd.Flags().IntVar(&depth, "depth", 0, "create a shallow clone with specified depth")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress output")
	cmd.Flags().StringVar(&keyPath, "key-path", "", "SSH key path (default: your ssh configuration)")
	cmd.Flags().StringVarP(&cloneDest, "dest", "d", "", "destination directory")
	if includeLayout {
		cmd.Flags().BoolVarP(&worktree, "worktree", "w", false, "clone as bare metadata repo with default worktrees")
		cmd.Flags().BoolVar(&worktree, "bare", false, "alias for --worktree")
	}
	cmd.Flags().StringVar(&featureWorktree, "feature", "", "create an additional feature worktree (worktree layout only)")
	cmd.Flags().StringVar(&featureBaseBranch, "feature-base", "", "base branch for --feature (defaults to repository default branch)")
}

func runAdd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := validateRepoInput(args[0]); err != nil {
		return err
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
	if err := validateRepoInput(args[0]); err != nil {
		return err
	}

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

	if len(args) == 2 {
		fmt.Fprintln(cmd.ErrOrStderr(), "A worktree argument to clone is deprecated; use ezgit wt add instead")
		worktreeName := args[1]
		return runCloneWithWorktree(cfg, args[0], worktreeName)
	}

	if worktree {
		return runWithDefaultCLIClonePlan(func() error {
			return runDirectClone(cfg, args[0], "", 0)
		})
	}

	return runDirectClone(cfg, args[0], "", 0)
}

func runWithDefaultCLIClonePlan(run func() error) error {
	plan := defaultCLICloneWorktreePlan()
	originalPlan := forcedClonePlan
	forcedClonePlan = &plan
	defer func() {
		forcedClonePlan = originalPlan
	}()
	return run()
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

func seedDefaultBranchLookupFromRepos(repos []github.Repo) {
	if len(repos) == 0 {
		return
	}

	homeDir, _ := os.UserHomeDir()
	if strings.TrimSpace(homeDir) == "" {
		return
	}

	seeded := make(map[string]string, len(repos))
	for _, repo := range repos {
		repoFullName := strings.TrimSpace(repo.FullName)
		branchName := strings.TrimSpace(repo.DefaultBranch)
		if repoFullName == "" || branchName == "" {
			continue
		}
		seeded[repoFullName] = branchName
	}

	if len(seeded) == 0 {
		return
	}

	defaultBranchLookupMu.Lock()
	defer defaultBranchLookupMu.Unlock()

	if defaultBranchLookupRepos == nil || defaultBranchLookupHome != homeDir {
		defaultBranchLookupRepos = make(map[string]string, len(seeded))
		defaultBranchLookupHome = homeDir
	}

	for repoFullName, branchName := range seeded {
		defaultBranchLookupRepos[repoFullName] = branchName
	}
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

	repo, err := parseRepository(repoInput)
	if err != nil {
		return err
	}
	repoURL := repo.URL
	gitMgr := git.New()
	dest := cloneDest
	if dest == "" {
		dest = repo.repoPath(cfg)
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
				registerRepoAndWorktreesWithZoxide(dest, quiet)
				return nil
			default:
				return fmt.Errorf("cancelled")
			}
		case existingRepoWorktree:
			if !worktree {
				if !quiet {
					fmt.Printf("Repository already exists at %s in worktree layout; skipping clone\n", dest)
				}
				registerRepoAndWorktreesWithZoxide(dest, quiet)
				return nil
			}
			didClone = false
		}
	}

	if didClone && keyPath != "" {
		if err := gitMgr.ValidateSSHKey(keyPath); err != nil {
			return fmt.Errorf("SSH key validation failed: %w", err)
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
		SSHKeyPath: keyPath,
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
		defaultPlan := defaultCLICloneWorktreePlan()
		createDefaultWorktree := defaultPlan.CreateDefault
		createReviewWorktree := defaultPlan.CreateReview
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
		registerRepoAndWorktreesWithZoxide(dest, quiet)
	} else {
		registerPathsWithZoxide([]string{dest}, quiet)
	}

	return nil
}

// runCloneWithWorktree handles `ezgit clone <repo> <worktreename>`.
// If the repo is already cloned, it just adds the worktree without prompts.
// If not yet cloned, it creates the default branch and specified worktrees
// without prompts.
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
				registerRepoAndWorktreesWithZoxide(dest, quiet)
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
		return existingRepoActionCancel, fmt.Errorf("destination %s is an existing non-worktree clone; re-run interactively, or run 'ezgit convert %s' first", dest, dest)
	}

	return promptExistingRegularRepoAction(os.Stdin, os.Stdout, dest)
}

func promptExistingRegularRepoAction(in io.Reader, out io.Writer, dest string) (existingRepoAction, error) {
	reader := bufio.NewReader(in)
	for {
		fmt.Fprintf(out, "Repository already exists at %s and is not in worktree layout. Choose action: [o]pen existing, [c]onvert to worktree, [x] cancel: ", dest)
		line, err := reader.ReadString('\n')
		input := strings.TrimSpace(line)
		if err != nil && (err != io.EOF || input == "") {
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
			fmt.Fprintln(out, "Please choose one of: o, c, x")
		}
	}
}

func registerRepoAndWorktreesWithZoxide(repoRoot string, quiet bool) {
	paths := []string{repoRoot}
	if worktrees, err := git.ListLinkedWorktrees(repoRoot); err == nil {
		for _, wt := range worktrees {
			paths = append(paths, wt.Path)
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
	repo, err := parseRepository(repoInput)
	if err != nil {
		return "", "", err
	}
	dest = cloneDest
	if dest == "" {
		dest = repo.repoPath(cfg)
	}

	metadataPath = filepath.Join(dest, ".git")
	return dest, metadataPath, nil
}
