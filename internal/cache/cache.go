package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kirksw/ezgit/internal/github"
)

const CacheDir = ".cache/ezgit"
const DefaultTTL = 24 * time.Hour
const PersonalCacheKey = "personal"

type OrgCache struct {
	cacheDir string
	ttl      time.Duration
}

type CacheMetadata struct {
	LastRefreshed       time.Time     `json:"last_refreshed"`
	TTL                 time.Duration `json:"ttl"`
	LatestRepoCreatedAt time.Time     `json:"latest_repo_created_at"`
}

type allReposSnapshot struct {
	repos     []github.Repo
	signature string
	expiresAt time.Time
}

var (
	allReposSnapshotMu sync.RWMutex
	allReposSnapshots  = make(map[string]allReposSnapshot)
)

func New() *OrgCache {
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, CacheDir)

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		cacheDir = os.TempDir()
	}

	return &OrgCache{
		cacheDir: cacheDir,
		ttl:      DefaultTTL,
	}
}

func (c *OrgCache) Get(org string) (*github.CachedOrg, error) {
	data, err := os.ReadFile(c.orgPath(org))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("org not cached: %s", org)
		}
		return nil, fmt.Errorf("failed to read cache: %w", err)
	}

	var cached github.CachedOrg
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache: %w", err)
	}

	if c.IsExpired(org) {
		return nil, fmt.Errorf("cache expired for org: %s", org)
	}

	return &cached, nil
}

func (c *OrgCache) Set(org string, repos []github.Repo) error {
	repos = sortReposByCreatedDesc(repos)

	cached := github.CachedOrg{
		Org:      org,
		Repos:    repos,
		CachedAt: time.Now(),
		TTL:      c.ttl.String(),
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(c.orgPath(org), data, 0644); err != nil {
		return fmt.Errorf("failed to write cache: %w", err)
	}

	metadata := CacheMetadata{
		LastRefreshed:       time.Now(),
		TTL:                 c.ttl,
		LatestRepoCreatedAt: latestRepoCreatedAt(repos),
	}

	metaData, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metaPath := c.metadataPath(org)
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	c.invalidateAllReposSnapshot()

	return nil
}

func (c *OrgCache) GetStale(org string) (*github.CachedOrg, error) {
	data, err := os.ReadFile(c.orgPath(org))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("org not cached: %s", org)
		}
		return nil, fmt.Errorf("failed to read cache: %w", err)
	}

	var cached github.CachedOrg
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache: %w", err)
	}

	return &cached, nil
}

func (c *OrgCache) GetLatestRepoCreatedAt(org string) (time.Time, error) {
	metadata, err := c.readMetadata(org)
	if err == nil && !metadata.LatestRepoCreatedAt.IsZero() {
		return metadata.LatestRepoCreatedAt, nil
	}

	cached, err := c.GetStale(org)
	if err != nil {
		return time.Time{}, err
	}

	return latestRepoCreatedAt(cached.Repos), nil
}

func (c *OrgCache) Refresh(org string, fetchRepos func() ([]github.Repo, error)) error {
	repos, err := fetchRepos()
	if err != nil {
		return fmt.Errorf("failed to fetch repos: %w", err)
	}

	return c.Set(org, repos)
}

func (c *OrgCache) Invalidate(org string) error {
	if err := os.Remove(c.orgPath(org)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache: %w", err)
	}

	if err := os.Remove(c.metadataPath(org)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	c.invalidateAllReposSnapshot()

	return nil
}

func (c *OrgCache) Search(pattern string) ([]github.Repo, error) {
	var allRepos []github.Repo

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		if strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}

		org := strings.TrimSuffix(entry.Name(), ".json")
		cached, err := c.Get(org)
		if err != nil {
			continue
		}

		for _, repo := range cached.Repos {
			if strings.Contains(repo.FullName, pattern) ||
				strings.Contains(repo.Name, pattern) ||
				strings.Contains(strings.ToLower(repo.Description), strings.ToLower(pattern)) {
				allRepos = append(allRepos, repo)
			}
		}
	}

	return allRepos, nil
}

func (c *OrgCache) ListAll() ([]string, error) {
	var orgs []string

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		if strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}

		org := strings.TrimSuffix(entry.Name(), ".json")
		orgs = append(orgs, org)
	}

	return orgs, nil
}

func (c *OrgCache) GetAllRepos() ([]github.Repo, error) {
	now := time.Now()
	signature, err := c.cacheSignature()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect cache dir: %w", err)
	}

	if cached, ok := c.getCachedAllRepos(signature, now); ok {
		return cached, nil
	}

	var allRepos []github.Repo
	seen := make(map[string]struct{})
	earliestExpiry := time.Time{}

	orgs, err := c.ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list orgs: %w", err)
	}

	for _, org := range orgs {
		metadata, err := c.readMetadata(org)
		if err != nil {
			continue
		}

		expiresAt := metadata.LastRefreshed.Add(metadata.TTL)
		if !expiresAt.After(now) {
			continue
		}
		if earliestExpiry.IsZero() || expiresAt.Before(earliestExpiry) {
			earliestExpiry = expiresAt
		}

		cached, err := c.GetStale(org)
		if err != nil {
			continue
		}

		for _, repo := range cached.Repos {
			if _, ok := seen[repo.FullName]; ok {
				continue
			}
			seen[repo.FullName] = struct{}{}
			allRepos = append(allRepos, repo)
		}
	}

	c.storeCachedAllRepos(signature, earliestExpiry, allRepos)

	return allRepos, nil
}

func (c *OrgCache) IsExpired(org string) bool {
	metadata, err := c.readMetadata(org)
	if err != nil {
		return true
	}
	if metadata.LastRefreshed.IsZero() {
		return true
	}

	return !time.Now().Before(metadata.LastRefreshed.Add(metadata.TTL))
}

func (c *OrgCache) readMetadata(org string) (CacheMetadata, error) {
	data, err := os.ReadFile(c.metadataPath(org))
	if err != nil {
		return CacheMetadata{}, err
	}

	var metadata CacheMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return CacheMetadata{}, err
	}

	if metadata.TTL <= 0 {
		metadata.TTL = DefaultTTL
	}

	return metadata, nil
}

func (c *OrgCache) cacheSignature() (string, error) {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%s:%d:%d", entry.Name(), info.Size(), info.ModTime().UnixNano()))
	}

	sort.Strings(parts)
	return strings.Join(parts, "|"), nil
}

func (c *OrgCache) getCachedAllRepos(signature string, now time.Time) ([]github.Repo, bool) {
	allReposSnapshotMu.RLock()
	snapshot, ok := allReposSnapshots[c.cacheDir]
	allReposSnapshotMu.RUnlock()
	if !ok || snapshot.signature != signature {
		return nil, false
	}
	if !snapshot.expiresAt.IsZero() && !now.Before(snapshot.expiresAt) {
		return nil, false
	}

	return cloneRepos(snapshot.repos), true
}

func (c *OrgCache) storeCachedAllRepos(signature string, expiresAt time.Time, repos []github.Repo) {
	allReposSnapshotMu.Lock()
	allReposSnapshots[c.cacheDir] = allReposSnapshot{
		repos:     cloneRepos(repos),
		signature: signature,
		expiresAt: expiresAt,
	}
	allReposSnapshotMu.Unlock()
}

func (c *OrgCache) invalidateAllReposSnapshot() {
	allReposSnapshotMu.Lock()
	delete(allReposSnapshots, c.cacheDir)
	allReposSnapshotMu.Unlock()
}

func cloneRepos(repos []github.Repo) []github.Repo {
	if len(repos) == 0 {
		return nil
	}

	cloned := make([]github.Repo, len(repos))
	copy(cloned, repos)
	return cloned
}

func (c *OrgCache) orgPath(org string) string {
	return filepath.Join(c.cacheDir, fmt.Sprintf("%s.json", org))
}

func (c *OrgCache) metadataPath(org string) string {
	return filepath.Join(c.cacheDir, fmt.Sprintf("%s.meta.json", org))
}

func sortReposByCreatedDesc(repos []github.Repo) []github.Repo {
	sorted := append([]github.Repo(nil), repos...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})
	return sorted
}

func latestRepoCreatedAt(repos []github.Repo) time.Time {
	var latest time.Time
	for _, repo := range repos {
		if repo.CreatedAt.After(latest) {
			latest = repo.CreatedAt
		}
	}
	return latest
}
