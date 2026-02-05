package github

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (g *GitHubClient) GetRepoSSHURL(repoFullName string) (string, error) {
	if len(g.token) == 0 {
		return fmt.Sprintf("git@github.com:%s.git", repoFullName), nil
	}

	url := fmt.Sprintf("%s/repos/%s", g.baseURL, repoFullName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	g.setAuth(req)

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch repo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("repo not found or access denied: %s", repoFullName)
	}

	var repo Repo
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return repo.SSHURL, nil
}
