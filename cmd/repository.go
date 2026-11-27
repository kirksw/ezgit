package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
	"github.com/kirksw/ezgit/internal/utils"
)

const githubHost = "github.com"

var remoteUserPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)
var hostPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

type repository struct {
	Host  string
	Owner string
	Name  string
	URL   string
}

func parseRepository(input string) (repository, error) {
	input = strings.TrimSpace(input)
	invalid := fmt.Errorf("invalid repository: expected owner/repo, host/owner/repo, or a Git URL")
	host, path, cloneURL := githubHost, input, ""
	switch {
	case strings.Contains(input, "://"):
		u, err := url.Parse(input)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "ssh") || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
			return repository{}, invalid
		}
		defaultPort := map[string]string{"https": "443", "http": "80", "ssh": "22"}
		if port := u.Port(); port != "" && port != defaultPort[u.Scheme] {
			return repository{}, fmt.Errorf("non-default Git URL ports are not supported by the host-based checkout layout")
		}
		if u.User != nil {
			if _, hasPassword := u.User.Password(); hasPassword || u.Scheme != "ssh" || !remoteUserPattern.MatchString(u.User.Username()) {
				return repository{}, invalid
			}
		}
		host, path, cloneURL = u.Hostname(), strings.TrimPrefix(u.Path, "/"), input
	case strings.Contains(input, ":"):
		remote, p, _ := strings.Cut(input, ":")
		user, h, ok := strings.Cut(remote, "@")
		if !ok || !remoteUserPattern.MatchString(user) {
			return repository{}, invalid
		}
		host, path, cloneURL = h, p, input
	default:
		if strings.Count(input, "/") == 2 {
			host, path, _ = strings.Cut(input, "/")
		}
	}
	host = strings.ToLower(host)
	path = strings.TrimSuffix(strings.TrimSuffix(path, "/"), ".git")
	owner, name, ok := strings.Cut(path, "/")
	if !ok || !hostPattern.MatchString(host) || strings.Contains(host, "..") || !githubOwnerPattern.MatchString(owner) || !isValidGitHubRepoName(name) {
		return repository{}, invalid
	}
	if cloneURL == "" {
		cloneURL = fmt.Sprintf("git@%s:%s/%s.git", host, owner, name)
	} else if !strings.HasSuffix(strings.TrimSuffix(cloneURL, "/"), ".git") {
		cloneURL = strings.TrimSuffix(cloneURL, "/") + ".git"
	}
	return repository{Host: host, Owner: owner, Name: name, URL: cloneURL}, nil
}

func (r repository) fullName() string { return r.Owner + "/" + r.Name }

func (r repository) repoPath(cfg *config.Config) string {
	canonical := filepath.Join(cfg.GetGitRoot(), r.Host, r.Owner, r.Name)
	if legacyRoot := cfg.GetCloneDir(); legacyRoot != "" && r.Host == githubHost {
		legacy := filepath.Join(legacyRoot, r.Owner, r.Name)
		if cfg.Git.Root == "" {
			return legacy
		}
		if _, err := os.Lstat(canonical); os.IsNotExist(err) {
			if _, err := os.Lstat(legacy); err == nil {
				return legacy
			}
		}
	}
	return canonical
}

func (r repository) worktreePath(cfg *config.Config, name string) string {
	return filepath.Join(cfg.GetGitRoot(), ".worktrees", r.Host, r.Owner, r.Name, name)
}

func localRepoMap(cfg *config.Config, repos []github.Repo) map[string]bool {
	if cfg.Git.Root == "" && cfg.GetCloneDir() != "" {
		return utils.BuildLocalRepoMap(cfg.GetCloneDir(), repos)
	}
	local := utils.BuildLocalRepoMap(filepath.Join(cfg.GetGitRoot(), githubHost), repos)
	if legacy := cfg.GetCloneDir(); legacy != "" {
		for name := range utils.BuildLocalRepoMap(legacy, repos) {
			local[name] = true
		}
	}
	return local
}

func localRepository(cfg *config.Config, repo repository) (string, existingRepoState, error) {
	path := repo.repoPath(cfg)
	state, err := detectExistingRepoState(path)
	if err != nil {
		return "", state, err
	}
	switch state {
	case existingRepoMissing:
		return "", state, fmt.Errorf("repository is not cloned: %s/%s; run ezgit clone first", repo.Host, repo.fullName())
	case existingRepoNonRepo:
		return "", state, fmt.Errorf("destination exists but is not a git repository: %s", path)
	}
	return path, state, nil
}
