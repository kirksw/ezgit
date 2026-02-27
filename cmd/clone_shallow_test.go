package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kirksw/ezgit/internal/config"
	"github.com/kirksw/ezgit/internal/github"
)

func TestExtractRepoFullName(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{
			name:   "owner slash repo",
			input:  "facebook/react",
			want:   "facebook/react",
			wantOK: true,
		},
		{
			name:   "ssh url",
			input:  "git@github.com:facebook/react.git",
			want:   "facebook/react",
			wantOK: true,
		},
		{
			name:   "https url",
			input:  "https://github.com/facebook/react",
			want:   "facebook/react",
			wantOK: true,
		},
		{
			name:   "invalid path",
			input:  "facebook/react/extra",
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractRepoFullName(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("extractRepoFullName() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("extractRepoFullName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseYesNoRequired(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue bool
		wantOK    bool
	}{
		{
			name:      "yes",
			input:     "y",
			wantValue: true,
			wantOK:    true,
		},
		{
			name:      "no",
			input:     "no",
			wantValue: false,
			wantOK:    true,
		},
		{
			name:      "empty is invalid",
			input:     "",
			wantValue: false,
			wantOK:    false,
		},
		{
			name:      "invalid",
			input:     "maybe",
			wantValue: false,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := parseYesNoRequired(tt.input)
			if gotOK != tt.wantOK {
				t.Fatalf("parseYesNoRequired() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotValue != tt.wantValue {
				t.Fatalf("parseYesNoRequired() value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestResolveDefaultBranchPrefersExplicitValue(t *testing.T) {
	originalLoader := loadRepoDefaultBranches
	defer func() {
		loadRepoDefaultBranches = originalLoader
		resetDefaultBranchLookupCache()
	}()

	resetDefaultBranchLookupCache()
	lookupCalls := 0
	loadRepoDefaultBranches = func() map[string]string {
		lookupCalls++
		return map[string]string{"acme/repo": "trunk"}
	}

	got := resolveDefaultBranch("acme/repo", "develop")
	if got != "develop" {
		t.Fatalf("resolveDefaultBranch() = %q, want %q", got, "develop")
	}
	if lookupCalls != 0 {
		t.Fatalf("lookupCalls = %d, want 0", lookupCalls)
	}
}

func TestResolveDefaultBranchLoadsRepoDefaultsOncePerHome(t *testing.T) {
	originalLoader := loadRepoDefaultBranches
	defer func() {
		loadRepoDefaultBranches = originalLoader
		resetDefaultBranchLookupCache()
	}()

	resetDefaultBranchLookupCache()
	lookupCalls := 0
	loadRepoDefaultBranches = func() map[string]string {
		lookupCalls++
		if lookupCalls == 1 {
			return map[string]string{"acme/repo": "trunk"}
		}
		return map[string]string{"acme/repo": "main"}
	}

	t.Setenv("HOME", t.TempDir())

	got := resolveDefaultBranch("acme/repo", "")
	if got != "trunk" {
		t.Fatalf("resolveDefaultBranch() = %q, want %q", got, "trunk")
	}

	got = resolveDefaultBranch("acme/repo", "")
	if got != "trunk" {
		t.Fatalf("resolveDefaultBranch() second call = %q, want %q", got, "trunk")
	}
	if lookupCalls != 1 {
		t.Fatalf("lookupCalls = %d, want 1", lookupCalls)
	}

	t.Setenv("HOME", t.TempDir())
	got = resolveDefaultBranch("acme/repo", "")
	if got != "main" {
		t.Fatalf("resolveDefaultBranch() after HOME change = %q, want %q", got, "main")
	}
	if lookupCalls != 2 {
		t.Fatalf("lookupCalls after HOME change = %d, want 2", lookupCalls)
	}

	fallback := resolveDefaultBranch("missing/repo", "")
	if fallback != "main" {
		t.Fatalf("resolveDefaultBranch() fallback = %q, want %q", fallback, "main")
	}
}

func TestSeedDefaultBranchLookupFromReposPreventsCacheReload(t *testing.T) {
	originalLoader := loadRepoDefaultBranches
	defer func() {
		loadRepoDefaultBranches = originalLoader
		resetDefaultBranchLookupCache()
	}()

	resetDefaultBranchLookupCache()
	lookupCalls := 0
	loadRepoDefaultBranches = func() map[string]string {
		lookupCalls++
		return map[string]string{"acme/repo": "main"}
	}

	t.Setenv("HOME", t.TempDir())
	seedDefaultBranchLookupFromRepos([]github.Repo{{FullName: "acme/repo", DefaultBranch: "trunk"}})

	got := resolveDefaultBranch("acme/repo", "")
	if got != "trunk" {
		t.Fatalf("resolveDefaultBranch() = %q, want %q", got, "trunk")
	}
	if lookupCalls != 0 {
		t.Fatalf("lookupCalls = %d, want 0", lookupCalls)
	}
}

func TestResolveCloneDepthForLargeRepo(t *testing.T) {
	cfg := &config.Config{
		Git: config.GitConfig{
			ShallowPromptThresholdKB: 1024,
		},
	}

	t.Run("uses shallow depth when user confirms", func(t *testing.T) {
		var out bytes.Buffer
		got := resolveCloneDepthForLargeRepo(
			cfg,
			"facebook/react",
			2048,
			0,
			strings.NewReader("y\n"),
			&out,
		)
		if got != recommendedShallowDepth {
			t.Fatalf("resolveCloneDepthForLargeRepo() = %d, want %d", got, recommendedShallowDepth)
		}
	})

	t.Run("keeps full clone when below threshold", func(t *testing.T) {
		var out bytes.Buffer
		got := resolveCloneDepthForLargeRepo(
			cfg,
			"facebook/react",
			512,
			0,
			strings.NewReader("y\n"),
			&out,
		)
		if got != 0 {
			t.Fatalf("resolveCloneDepthForLargeRepo() = %d, want 0", got)
		}
	})

	t.Run("keeps explicit depth", func(t *testing.T) {
		var out bytes.Buffer
		got := resolveCloneDepthForLargeRepo(
			cfg,
			"facebook/react",
			2048,
			3,
			strings.NewReader("y\n"),
			&out,
		)
		if got != 3 {
			t.Fatalf("resolveCloneDepthForLargeRepo() = %d, want 3", got)
		}
	})
}

func TestResolveClonePathsWorktree(t *testing.T) {
	cloneTarget, metadataPath := resolveClonePaths("/tmp/example", true)
	want := "/tmp/example/.git"
	if cloneTarget != want {
		t.Fatalf("cloneTarget=%q, want %q", cloneTarget, want)
	}
	if metadataPath != want {
		t.Fatalf("metadataPath=%q, want %q", metadataPath, want)
	}
}

func TestResolveFeatureWorktreeConfigDefaultsBaseBranch(t *testing.T) {
	feature, base, err := resolveFeatureWorktreeConfig("main", "feature/test", "")
	if err != nil {
		t.Fatalf("resolveFeatureWorktreeConfig() error = %v", err)
	}
	if feature != "feature/test" {
		t.Fatalf("feature=%q, want %q", feature, "feature/test")
	}
	if base != "main" {
		t.Fatalf("base=%q, want %q", base, "main")
	}
}

func TestResolveFeatureWorktreeConfigRequiresFeatureWhenBaseProvided(t *testing.T) {
	if _, _, err := resolveFeatureWorktreeConfig("main", "", "develop"); err == nil {
		t.Fatal("expected error when --feature-base is set without --feature")
	}
}

func TestResolveFeatureWorktreeConfigRejectsReservedReviewName(t *testing.T) {
	if _, _, err := resolveFeatureWorktreeConfig("main", "review", "main"); err == nil {
		t.Fatal("expected error for reserved review worktree name")
	}
}

func TestResolveClonePathsStandardClone(t *testing.T) {
	cloneTarget, metadataPath := resolveClonePaths("/tmp/example", false)
	want := "/tmp/example"
	if cloneTarget != want {
		t.Fatalf("cloneTarget=%q, want %q", cloneTarget, want)
	}
	if metadataPath != want {
		t.Fatalf("metadataPath=%q, want %q", metadataPath, want)
	}
}

func TestResolveOpenTargetPathWithWorktree(t *testing.T) {
	got := resolveOpenTargetPath("/tmp/repo", "main")
	want := "/tmp/repo/main"
	if got != want {
		t.Fatalf("resolveOpenTargetPath()=%q, want %q", got, want)
	}
}

func TestResolveOpenTargetPathWithoutWorktree(t *testing.T) {
	got := resolveOpenTargetPath("/tmp/repo", "")
	want := "/tmp/repo"
	if got != want {
		t.Fatalf("resolveOpenTargetPath()=%q, want %q", got, want)
	}
}

func TestResolveRepoPathsUsesCloneDir(t *testing.T) {
	cfg := &config.Config{
		Git: config.GitConfig{CloneDir: "/tmp/clones"},
	}

	dest, metadata, err := resolveRepoPaths(cfg, "lunarway/hubble-cli")
	if err != nil {
		t.Fatalf("resolveRepoPaths() error = %v", err)
	}
	if dest != "/tmp/clones/lunarway/hubble-cli" {
		t.Fatalf("dest=%q, want %q", dest, "/tmp/clones/lunarway/hubble-cli")
	}
	if metadata != "/tmp/clones/lunarway/hubble-cli/.git" {
		t.Fatalf("metadata=%q, want %q", metadata, "/tmp/clones/lunarway/hubble-cli/.git")
	}
}

func TestAddWorktreeToRepoRejectsEmptyName(t *testing.T) {
	err := addWorktreeToRepo(nil, "lunarway/hubble-cli", "/tmp/repo", "/tmp/repo/.git", "   ")
	if err == nil {
		t.Fatal("expected error for empty worktree name")
	}
}

func TestRegisterPathsWithZoxideDedupesAndNormalizes(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	original := runZoxideAdd
	defer func() { runZoxideAdd = original }()

	var added []string
	runZoxideAdd = func(path string) error {
		added = append(added, path)
		return nil
	}

	registerPathsWithZoxide([]string{
		repoPath,
		filepath.Join(repoPath, "..", "repo"),
		"",
		"   ",
	}, true)

	if len(added) != 1 {
		t.Fatalf("len(added)=%d, want 1", len(added))
	}
	if added[0] != absRepoPath {
		t.Fatalf("added[0]=%q, want %q", added[0], absRepoPath)
	}
}

func TestRegisterPathsWithZoxideContinuesAfterErrors(t *testing.T) {
	tempDir := t.TempDir()
	first := filepath.Join(tempDir, "one")
	second := filepath.Join(tempDir, "two")
	if err := os.MkdirAll(first, 0755); err != nil {
		t.Fatalf("mkdir first: %v", err)
	}
	if err := os.MkdirAll(second, 0755); err != nil {
		t.Fatalf("mkdir second: %v", err)
	}

	original := runZoxideAdd
	defer func() { runZoxideAdd = original }()

	var calls int
	runZoxideAdd = func(path string) error {
		calls++
		if calls == 1 {
			return fmt.Errorf("boom")
		}
		return nil
	}

	registerPathsWithZoxide([]string{first, second}, true)

	if calls != 2 {
		t.Fatalf("calls=%d, want 2", calls)
	}
}
