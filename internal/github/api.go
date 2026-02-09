package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GitHubClient struct {
	token   string
	client  *http.Client
	baseURL string
}

type Repo struct {
	Name            string    `json:"name"`
	FullName        string    `json:"full_name"`
	URL             string    `json:"clone_url"`
	SSHURL          string    `json:"ssh_url"`
	Size            int       `json:"size"`
	CreatedAt       time.Time `json:"created_at"`
	Private         bool      `json:"private"`
	DefaultBranch   string    `json:"default_branch"`
	UpdatedAt       time.Time `json:"updated_at"`
	Description     string    `json:"description"`
	Language        string    `json:"language"`
	StargazersCount int       `json:"stargazers_count"`
}

func NewClient(token string) *GitHubClient {
	return &GitHubClient{
		token: token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.github.com",
	}
}

func (g *GitHubClient) FetchOrgRepos(org string) ([]Repo, error) {
	url := fmt.Sprintf("%s/orgs/%s/repos?per_page=100&sort=created&direction=desc", g.baseURL, org)
	return g.fetchAllRepos(url)
}

func (g *GitHubClient) FetchPrivateRepos() ([]Repo, error) {
	url := fmt.Sprintf("%s/user/repos?per_page=100&type=owner&sort=created&direction=desc", g.baseURL)
	return g.fetchAllRepos(url)
}

func (g *GitHubClient) FetchOrgReposCreatedAfter(org string, createdAfter time.Time) ([]Repo, error) {
	url := fmt.Sprintf("%s/orgs/%s/repos?per_page=100&sort=created&direction=desc", g.baseURL, org)
	return g.fetchReposCreatedAfter(url, createdAfter)
}

func (g *GitHubClient) FetchPrivateReposCreatedAfter(createdAfter time.Time) ([]Repo, error) {
	url := fmt.Sprintf("%s/user/repos?per_page=100&type=owner&sort=created&direction=desc", g.baseURL)
	return g.fetchReposCreatedAfter(url, createdAfter)
}

func (g *GitHubClient) ValidateToken() error {
	url := fmt.Sprintf("%s/user", g.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	g.setAuth(req)

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("invalid token: %s", string(body))
	}

	return nil
}

func (g *GitHubClient) fetchAllRepos(url string) ([]Repo, error) {
	var allRepos []Repo

	for url != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		g.setAuth(req)

		resp, err := g.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch repos: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
		}

		var repos []Repo
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		allRepos = append(allRepos, repos...)

		linkHeader := resp.Header.Get("Link")
		url = extractNextURL(linkHeader)
	}

	return allRepos, nil
}

func (g *GitHubClient) fetchReposCreatedAfter(url string, createdAfter time.Time) ([]Repo, error) {
	if createdAfter.IsZero() {
		return g.fetchAllRepos(url)
	}

	var allRepos []Repo

	for url != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		g.setAuth(req)

		resp, err := g.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch repos: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
		}

		var repos []Repo
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		linkHeader := resp.Header.Get("Link")
		resp.Body.Close()

		stop := false
		for _, repo := range repos {
			// Repos are requested as sort=created&direction=desc, so once the
			// threshold is reached, all remaining pages are older.
			if !repo.CreatedAt.After(createdAfter) {
				stop = true
				break
			}
			allRepos = append(allRepos, repo)
		}

		if stop {
			break
		}

		url = extractNextURL(linkHeader)
	}

	return allRepos, nil
}

type Branch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

func (g *GitHubClient) FetchBranches(repoFullName string) ([]Branch, error) {
	url := fmt.Sprintf("%s/repos/%s/branches?per_page=100", g.baseURL, repoFullName)

	var allBranches []Branch

	for url != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		g.setAuth(req)

		resp, err := g.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch branches: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
		}

		var branches []Branch
		if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		allBranches = append(allBranches, branches...)

		linkHeader := resp.Header.Get("Link")
		url = extractNextURL(linkHeader)
	}

	return allBranches, nil
}

func (g *GitHubClient) setAuth(req *http.Request) {
	if g.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", g.token))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
}

func extractNextURL(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		if strings.Contains(link, `rel="next"`) {
			parts := strings.Split(link, ";")
			if len(parts) > 0 {
				url := strings.TrimSpace(parts[0])
				url = strings.Trim(url, "<>")
				return url
			}
		}
	}

	return ""
}
