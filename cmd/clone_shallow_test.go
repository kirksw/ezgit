package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kirksw/ezgit/internal/config"
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
