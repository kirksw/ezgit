package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/git"
	"github.com/spf13/cobra"
)

var worktreeCmd = &cobra.Command{
	Use:     "worktree",
	Aliases: []string{"wt"},
	Short:   "Manage repository worktrees",
}

var worktreeAddCmd = &cobra.Command{
	Use:   "add <repo> <worktreename>",
	Short: "Add a worktree to a repository",
	Args:  cobra.ExactArgs(2),
	RunE:  runAdd,
}

var worktreePruneCmd = &cobra.Command{
	Use:   "prune <repo>",
	Short: "Find and remove old worktrees",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorktreePrune,
}

var (
	worktreePruneOlderThan = "14d"
	worktreePruneApply     bool
)

type staleWorktree struct {
	Path         string
	Branch       string
	LastModified time.Time
}

func init() {
	rootCmd.AddCommand(worktreeCmd)
	worktreeCmd.AddCommand(worktreeAddCmd, worktreePruneCmd)
	worktreePruneCmd.Flags().StringVar(&worktreePruneOlderThan, "older-than", "14d", "minimum worktree age (for example 14d or 48h)")
	worktreePruneCmd.Flags().BoolVar(&worktreePruneApply, "apply", false, "remove listed worktrees, including dirty worktrees")
}

func runWorktreePrune(cmd *cobra.Command, args []string) error {
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
	switch state {
	case existingRepoMissing:
		return fmt.Errorf("repository is not cloned: %s", repoFullName)
	case existingRepoRegular:
		return fmt.Errorf("repository does not use worktree layout: %s", repoFullName)
	case existingRepoNonRepo:
		return fmt.Errorf("destination exists but is not a git repository: %s", repoPath)
	}

	olderThan, err := parseWorktreeAge(worktreePruneOlderThan)
	if err != nil {
		return fmt.Errorf("invalid --older-than: %w", err)
	}

	gitMgr := git.New()
	worktrees, err := gitMgr.ListWorktreeDetails(repoPath)
	if err != nil {
		return fmt.Errorf("failed to list worktrees for %s: %w", repoFullName, err)
	}
	candidates, err := findStaleWorktrees(worktrees, time.Now().Add(-olderThan))
	if err != nil {
		return fmt.Errorf("failed to inspect worktrees for %s: %w", repoFullName, err)
	}
	if len(candidates) == 0 {
		fmt.Printf("No worktrees older than %s found for %s\n", formatWorktreeAge(olderThan), repoFullName)
		return nil
	}

	for _, candidate := range candidates {
		age := time.Since(candidate.LastModified).Round(time.Minute)
		fmt.Printf("%s\t%s\t%s old\n", candidate.Branch, candidate.Path, formatWorktreeAge(age))
	}
	if !worktreePruneApply {
		fmt.Println("Preview only; re-run with --apply to remove these worktrees")
		return nil
	}

	for _, candidate := range candidates {
		if err := gitMgr.RemoveWorktree(repoPath, candidate.Path, true); err != nil {
			return fmt.Errorf("failed to remove worktree %s: %w", candidate.Path, err)
		}
		fmt.Printf("Removed %s\n", candidate.Path)
	}
	return nil
}

func parseWorktreeAge(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "d") {
		daysText := strings.TrimSuffix(value, "d")
		days, err := time.ParseDuration(daysText + "h")
		if err != nil {
			return 0, fmt.Errorf("expected a duration such as 14d or 48h")
		}
		value = (days * 24).String()
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("expected a duration such as 14d or 48h")
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be greater than zero")
	}
	return duration, nil
}

func formatWorktreeAge(duration time.Duration) string {
	if duration < 0 {
		duration = -duration
	}
	duration = duration.Round(time.Minute)
	days := duration / (24 * time.Hour)
	duration %= 24 * time.Hour
	hours := duration / time.Hour
	duration %= time.Hour
	minutes := duration / time.Minute

	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	return strings.Join(parts, " ")
}

func findStaleWorktrees(worktrees []git.Worktree, cutoff time.Time) ([]staleWorktree, error) {
	candidates := make([]staleWorktree, 0)
	for _, worktree := range worktrees {
		branch := strings.TrimSpace(worktree.Branch)
		worktreeName := filepath.Base(filepath.Clean(worktree.Path))
		if branch == "main" || branch == "master" || worktreeName == "main" || worktreeName == "master" || worktreeName == ".git" {
			continue
		}
		info, err := os.Stat(worktree.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to inspect %s: %w", worktree.Path, err)
		}
		if !info.IsDir() {
			continue
		}

		lastModified, err := newestWorktreeModification(worktree.Path)
		if err != nil {
			return nil, err
		}
		if lastModified.After(cutoff) {
			continue
		}
		if branch == "" {
			branch = "(detached)"
		}
		candidates = append(candidates, staleWorktree{
			Path:         worktree.Path,
			Branch:       branch,
			LastModified: lastModified,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LastModified.Before(candidates[j].LastModified)
	})
	return candidates, nil
}

func newestWorktreeModification(root string) (time.Time, error) {
	var newest time.Time
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root && entry.Name() == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to scan %s: %w", root, err)
	}
	return newest, nil
}
