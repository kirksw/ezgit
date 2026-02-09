package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Organizations OrganizationConfig `toml:"organizations"`
	Repos         RepoConfig         `toml:"repos"`
	GitHub        GitHubConfig       `toml:"github"`
	Git           GitConfig          `toml:"git"`
}

type OrganizationConfig struct {
	Orgs []string `toml:"orgs"`
}

type RepoConfig struct {
	Private []string `toml:"private"`
}

type GitHubConfig struct {
	Token string `toml:"token"`
}

type GitConfig struct {
	CloneDir                 string `toml:"clone_dir"`
	Worktree                 bool   `toml:"worktree"`
	SeshOpen                 bool   `toml:"sesh_open"`
	OpenCommand              string `toml:"open_command"`
	ShallowPromptThresholdKB int    `toml:"shallow_prompt_threshold_kb"`
}

func Load(path string) (*Config, error) {
	configPath, err := FindConfigPath(path)
	if err != nil {
		return &Config{}, nil
	}

	return LoadFile(configPath)
}

func FindConfigPath(path string) (string, error) {
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	configPaths := []string{
		"./config.toml",
		filepath.Join(homeDir, ".config", "ezgit", "config.toml"),
		filepath.Join(homeDir, ".ezgit.toml"),
	}

	for _, p := range configPaths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("no config file found")
}

func LoadFile(path string) (*Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.GitHub.Token == "" {
		cfg.GitHub.Token = os.Getenv("GITHUB_TOKEN")
	}

	return &cfg, nil
}

func (c *Config) GetOrganizations() []string {
	return c.Organizations.Orgs
}

func (c *Config) GetPrivateRepos() []string {
	return c.Repos.Private
}

var (
	ghTokenCache   string
	ghTokenCached  bool
	ghTokenCacheMu sync.RWMutex
)

func getGitHubCLIAuthToken() (string, error) {
	ghTokenCacheMu.RLock()
	if ghTokenCached {
		token := ghTokenCache
		ghTokenCacheMu.RUnlock()
		return token, nil
	}
	ghTokenCacheMu.RUnlock()

	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	token := strings.TrimSpace(string(output))

	ghTokenCacheMu.Lock()
	ghTokenCache = token
	ghTokenCached = true
	ghTokenCacheMu.Unlock()

	return token, nil
}

func (c *Config) GetGitHubToken() string {
	token, err := getGitHubCLIAuthToken()
	if err == nil && token != "" {
		return token
	}

	if c.GitHub.Token != "" {
		return c.GitHub.Token
	}

	return os.Getenv("GITHUB_TOKEN")
}

func (c *Config) GetCloneDir() string {
	dir := c.Git.CloneDir
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	return dir
}

func ParseOwnerRepo(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if strings.Contains(input, "/") {
		parts := strings.Split(input, "/")
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}
	return "", "", fmt.Errorf("invalid owner/repo format")
}
