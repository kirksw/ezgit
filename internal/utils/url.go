package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var urlPattern = regexp.MustCompile(`^git@github\.com:([^/]+)/([^/]+)\.git$`)

func ParseRepoIdentifier(input string) (string, string, error) {
	input = strings.TrimSpace(input)

	if strings.HasPrefix(input, "git@") {
		matches := urlPattern.FindStringSubmatch(input)
		if len(matches) == 3 {
			return matches[1], matches[2], nil
		}
		return "", "", fmt.Errorf("invalid SSH URL format")
	}

	if strings.Contains(input, "/") {
		parts := strings.Split(input, "/")
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("invalid repo identifier: %s (expected owner/repo or git@github.com:owner/repo.git)", input)
}

func BuildSSHURL(owner, repo string) string {
	return fmt.Sprintf("git@github.com:%s/%s.git", owner, repo)
}
