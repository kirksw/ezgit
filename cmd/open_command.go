package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kirksw/ezgit/internal/config"
)

const defaultOpenCommandTemplate = `sesh connect "$absPath"`

type openCommandContext struct {
	Org          string
	Repo         string
	Worktree     string
	AbsPath      string
	RepoPath     string
	OrgRepo      string
	RepoFullName string
}

func resolveOpenCommandTemplate(cfg *config.Config) string {
	command := strings.TrimSpace(cfg.Git.OpenCommand)
	if command == "" {
		return defaultOpenCommandTemplate
	}
	return command
}

func buildOpenCommandContext(cfg *config.Config, repoFullName string, selectedWorktree string) (openCommandContext, error) {
	cloneDir := cfg.GetCloneDir()
	if cloneDir == "" {
		return openCommandContext{}, fmt.Errorf("clone_dir must be set in config to use 'ezgit open'")
	}

	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return openCommandContext{}, fmt.Errorf("invalid repo format: %s", repoFullName)
	}

	org := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	worktree := strings.TrimSpace(selectedWorktree)
	orgRepo := filepath.ToSlash(filepath.Join(org, repo))
	repoRootPath := filepath.Join(cloneDir, org, repo)

	if worktree == "" {
		// The worktree layout keeps only git metadata at the repo root, so
		// opening it directly is not useful. Fall back to the default-branch
		// worktree when the user did not pick one.
		worktree = resolveDefaultBranchWorktree(repoFullName, repoRootPath)
	}

	repoPath := orgRepo
	if worktree != "" {
		repoPath = filepath.ToSlash(filepath.Join(orgRepo, worktree))
	}

	absPath := resolveOpenTargetPath(repoRootPath, worktree)

	return openCommandContext{
		Org:          org,
		Repo:         repo,
		Worktree:     worktree,
		AbsPath:      absPath,
		RepoPath:     repoPath,
		OrgRepo:      orgRepo,
		RepoFullName: repoFullName,
	}, nil
}

// resolveDefaultBranchWorktree returns the worktree directory name to open
// when the user did not select one. Repos using the worktree layout store
// only git metadata at the root, so the default-branch worktree is opened
// instead. Regular clones and missing repos return "" (repo root is opened).
func resolveDefaultBranchWorktree(repoFullName, repoRootPath string) string {
	state, err := detectExistingRepoState(repoRootPath)
	if err != nil || state != existingRepoWorktree {
		return ""
	}

	defaultBranch := resolveDefaultBranch(repoFullName, "")
	candidates := make([]string, 0, 3)
	for _, branch := range []string{defaultBranch, "main", "master"} {
		if branch == "" {
			continue
		}
		candidates = append(candidates, branch)
	}

	for _, branch := range candidates {
		if info, err := os.Stat(filepath.Join(repoRootPath, branch)); err == nil && info.IsDir() {
			return branch
		}
	}

	// Fallback: first worktree-shaped subdirectory.
	entries, err := os.ReadDir(repoRootPath)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".git" {
			continue
		}
		return entry.Name()
	}
	return ""
}

func runOpenCommand(cfg *config.Config, repoFullName string, selectedWorktree string) error {
	ctx, err := buildOpenCommandContext(cfg, repoFullName, selectedWorktree)
	if err != nil {
		return err
	}

	command := resolveOpenCommandTemplate(cfg)
	cmd := exec.Command("bash", "-c", command)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("org=%s", ctx.Org),
		fmt.Sprintf("repo=%s", ctx.Repo),
		fmt.Sprintf("worktree=%s", ctx.Worktree),
		fmt.Sprintf("absPath=%s", ctx.AbsPath),
		fmt.Sprintf("repoPath=%s", ctx.RepoPath),
		fmt.Sprintf("orgRepo=%s", ctx.OrgRepo),
		fmt.Sprintf("repoFullName=%s", ctx.RepoFullName),
		fmt.Sprintf("ORG=%s", ctx.Org),
		fmt.Sprintf("REPO=%s", ctx.Repo),
		fmt.Sprintf("WORKTREE=%s", ctx.Worktree),
		fmt.Sprintf("ABS_PATH=%s", ctx.AbsPath),
		fmt.Sprintf("REPO_PATH=%s", ctx.RepoPath),
		fmt.Sprintf("ORG_REPO=%s", ctx.OrgRepo),
		fmt.Sprintf("REPO_FULL_NAME=%s", ctx.RepoFullName),
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open command failed: %w", err)
	}
	return nil
}
