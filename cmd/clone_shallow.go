package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kirksw/ezgit/internal/cache"
	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
)

const recommendedShallowDepth = 1

func isInteractiveStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}

func resolveCloneDepthForLargeRepo(
	cfg *config.Config,
	repoInput string,
	repoSizeHintKB int,
	currentDepth int,
	in io.Reader,
	out io.Writer,
) int {
	if currentDepth > 0 {
		return currentDepth
	}

	thresholdKB := cfg.Git.ShallowPromptThresholdKB
	if thresholdKB <= 0 {
		return currentDepth
	}

	repoFullName, ok := extractRepoFullName(repoInput)
	if !ok {
		return currentDepth
	}

	repoSizeKB := repoSizeHintKB
	if repoSizeKB <= 0 {
		repoSizeKB = lookupCachedRepoSizeKB(repoFullName)
	}

	if repoSizeKB <= 0 {
		client := github.NewClient(cfg.GetGitHubToken())
		repo, err := client.GetRepo(repoFullName)
		if err == nil {
			repoSizeKB = repo.Size
		}
	}

	if repoSizeKB < thresholdKB {
		return currentDepth
	}

	useShallow, err := promptShallowCloneRecommendation(in, out, repoFullName, repoSizeKB, thresholdKB)
	if err != nil || !useShallow {
		return currentDepth
	}

	return recommendedShallowDepth
}

func lookupCachedRepoSizeKB(repoFullName string) int {
	c := cache.New()
	repos, err := c.GetAllRepos()
	if err != nil {
		return 0
	}

	for _, repo := range repos {
		if repo.FullName == repoFullName {
			return repo.Size
		}
	}

	return 0
}

func promptShallowCloneRecommendation(
	in io.Reader,
	out io.Writer,
	repoFullName string,
	repoSizeKB int,
	thresholdKB int,
) (bool, error) {
	reader := bufio.NewReader(in)
	repoSizeMB := float64(repoSizeKB) / 1024.0
	thresholdMB := float64(thresholdKB) / 1024.0

	for {
		fmt.Fprintf(
			out,
			"The repo you are cloning (%s) is large (%.1f MB >= %.1f MB), so we recommend a shallow clone with --depth %d. Use shallow clone? [y/n]: ",
			repoFullName,
			repoSizeMB,
			thresholdMB,
			recommendedShallowDepth,
		)

		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if answer, ok := parseYesNoRequired(input); ok {
					return answer, nil
				}
			}
			return false, err
		}

		if answer, ok := parseYesNoRequired(input); ok {
			return answer, nil
		}

		fmt.Fprintln(out, "Please answer y or n.")
	}
}

func parseYesNoRequired(input string) (bool, bool) {
	value := strings.ToLower(strings.TrimSpace(input))
	switch value {
	case "y", "yes":
		return true, true
	case "n", "no":
		return false, true
	default:
		return false, false
	}
}

func extractRepoFullName(input string) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", false
	}

	switch {
	case strings.HasPrefix(trimmed, "git@github.com:"):
		trimmed = strings.TrimPrefix(trimmed, "git@github.com:")
	case strings.HasPrefix(trimmed, "https://github.com/"):
		trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	case strings.HasPrefix(trimmed, "http://github.com/"):
		trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	}

	trimmed = strings.TrimSuffix(trimmed, ".git")
	trimmed = strings.TrimSuffix(trimmed, "/")

	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}

	return parts[0] + "/" + parts[1], true
}
