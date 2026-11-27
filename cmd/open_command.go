package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
)

const defaultOpenCommandTemplate = `sesh connect "$absPath"`

type openCommandContext struct {
	Host         string
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

func buildOpenCommandContext(cfg *config.Config, repoInput string, selectedWorktree string) (openCommandContext, error) {
	repo, err := parseRepository(repoInput)
	if err != nil {
		return openCommandContext{}, err
	}
	repoRoot, state, err := localRepository(cfg, repo)
	if err != nil {
		return openCommandContext{}, err
	}
	worktree := strings.TrimSpace(selectedWorktree)
	target := repoRoot
	if worktree != "" || state == existingRepoWorktree {
		linked, err := git.ListLinkedWorktrees(repoRoot)
		if err != nil {
			return openCommandContext{}, fmt.Errorf("failed to list worktrees: %w", err)
		}
		selectors := git.WorktreeSelectors(repoRoot, linked)
		if worktree == "" {
			if len(linked) == 0 {
				return openCommandContext{}, fmt.Errorf("no linked checkout in %s; run ezgit wt add first", repoRoot)
			}
			worktree = selectors[0]
		defaultCheckout:
			for _, name := range []string{resolveDefaultBranch(repo.fullName(), ""), "main", "master"} {
				for _, selector := range selectors {
					if selector == name {
						worktree = selector
						break defaultCheckout
					}
				}
			}
		}
		target = ""
		for i, wt := range linked {
			if worktree == selectors[i] || worktree == wt.Path {
				target = wt.Path
				worktree = git.WorktreeName(repoRoot, wt.Path)
				break
			}
		}
		if target == "" {
			for _, wt := range linked {
				if worktree == git.WorktreeName(repoRoot, wt.Path) {
					return openCommandContext{}, fmt.Errorf("worktree name %q is ambiguous; use a path from ezgit list worktrees", worktree)
				}
			}
			return openCommandContext{}, fmt.Errorf("worktree %q does not exist; run ezgit wt add first", worktree)
		}
	}
	absPath, err := filepath.Abs(target)
	if err != nil {
		return openCommandContext{}, fmt.Errorf("failed to resolve checkout path: %w", err)
	}
	return openCommandContext{
		Host: repo.Host, Org: repo.Owner, Repo: repo.Name, Worktree: worktree, AbsPath: absPath,
		RepoPath: filepath.ToSlash(filepath.Join(repo.fullName(), worktree)), OrgRepo: repo.fullName(), RepoFullName: repo.fullName(),
	}, nil
}

func runOpenCommand(cfg *config.Config, repoFullName string, selectedWorktree string) error {
	ctx, err := buildOpenCommandContext(cfg, repoFullName, selectedWorktree)
	if err != nil {
		return err
	}

	command := resolveOpenCommandTemplate(cfg)
	cmd := exec.Command("bash", "-c", command)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("host=%s", ctx.Host),
		fmt.Sprintf("org=%s", ctx.Org),
		fmt.Sprintf("repo=%s", ctx.Repo),
		fmt.Sprintf("worktree=%s", ctx.Worktree),
		fmt.Sprintf("absPath=%s", ctx.AbsPath),
		fmt.Sprintf("repoPath=%s", ctx.RepoPath),
		fmt.Sprintf("orgRepo=%s", ctx.OrgRepo),
		fmt.Sprintf("repoFullName=%s", ctx.RepoFullName),
		fmt.Sprintf("HOST=%s", ctx.Host),
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
