package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid format",
			input:     "facebook/react",
			wantOwner: "facebook",
			wantRepo:  "react",
			wantErr:   false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseOwnerRepo(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOwnerRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("ParseOwnerRepo() = (%v, %v), want (%v, %v)", owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

func TestFindConfigPath(t *testing.T) {
	homeDir := t.TempDir()

	tests := []struct {
		name     string
		setup    func() string
		wantPath string
		wantErr  bool
	}{
		{
			name: "explicit path exists",
			setup: func() string {
				return "/tmp/test-config.toml"
			},
			wantPath: "",
			wantErr:  true,
		},
		{
			name: "find in config dir",
			setup: func() string {
				configDir := filepath.Join(homeDir, ".config", "ezgit")
				os.MkdirAll(configDir, 0755)
				configPath := filepath.Join(configDir, "config.toml")
				os.WriteFile(configPath, []byte("[organizations]\norgs = []"), 0644)
				return ""
			},
			wantPath: filepath.Join(homeDir, ".config", "ezgit", "config.toml"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldHome := os.Getenv("HOME")
			os.Setenv("HOME", homeDir)
			defer os.Setenv("HOME", oldHome)

			explicitPath := tt.setup()

			path, err := FindConfigPath(explicitPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindConfigPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && path != tt.wantPath {
				t.Errorf("FindConfigPath() = %v, want %v", path, tt.wantPath)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	configContent := `[organizations]
orgs = ["facebook", "google"]

[repos]
private = ["my-org/secret"]

[git]
clone_dir = "~/git/github.com"
worktree = true
sesh_open = false
open_command = "sesh connect \"$repoPath\""
shallow_prompt_threshold_kb = 204800
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	orgs := cfg.GetOrganizations()
	if len(orgs) != 2 || orgs[0] != "facebook" || orgs[1] != "google" {
		t.Errorf("GetOrganizations() = %v, want [facebook google]", orgs)
	}

	private := cfg.GetPrivateRepos()
	if len(private) != 1 || private[0] != "my-org/secret" {
		t.Errorf("GetPrivateRepos() = %v, want [my-org/secret]", private)
	}

	cloneDir := cfg.GetCloneDir()
	homeDir, _ := os.UserHomeDir()
	expectedCloneDir := filepath.Join(homeDir, "git/github.com")
	if cloneDir != expectedCloneDir {
		t.Errorf("GetCloneDir() = %v, want %v", cloneDir, expectedCloneDir)
	}

	if !cfg.Git.Worktree {
		t.Errorf("Git.Worktree = %v, want true", cfg.Git.Worktree)
	}

	if cfg.Git.SeshOpen {
		t.Errorf("Git.SeshOpen = %v, want false", cfg.Git.SeshOpen)
	}

	if cfg.Git.OpenCommand != "sesh connect \"$repoPath\"" {
		t.Errorf("Git.OpenCommand = %q, want %q", cfg.Git.OpenCommand, "sesh connect \"$repoPath\"")
	}

	if cfg.Git.ShallowPromptThresholdKB != 204800 {
		t.Errorf("Git.ShallowPromptThresholdKB = %v, want 204800", cfg.Git.ShallowPromptThresholdKB)
	}
}

func TestGetGitHubToken(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldPath := os.Getenv("PATH")

	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("PATH", oldPath)
		os.Unsetenv("GITHUB_TOKEN")
		ghTokenCache = ""
		ghTokenCached = false
	})

	t.Run("uses gh cli token when available", func(t *testing.T) {
		ghTokenCache = "gh_test_token_12345"
		ghTokenCached = true

		cfg := &Config{}
		token := cfg.GetGitHubToken()
		if token != "gh_test_token_12345" {
			t.Errorf("GetGitHubToken() = %v, want gh_test_token_12345", token)
		}
	})

	t.Run("falls back to config token when gh not available", func(t *testing.T) {
		ghTokenCache = ""
		ghTokenCached = false
		os.Setenv("PATH", "")

		os.Setenv("GITHUB_TOKEN", "")
		cfg := &Config{
			GitHub: GitHubConfig{Token: "config_token"},
		}
		token := cfg.GetGitHubToken()
		if token != "config_token" {
			t.Errorf("GetGitHubToken() = %v, want config_token", token)
		}
	})

	t.Run("falls back to env var when gh not available", func(t *testing.T) {
		ghTokenCache = ""
		ghTokenCached = false
		os.Setenv("PATH", "")

		os.Setenv("GITHUB_TOKEN", "env_token")
		cfg := &Config{}
		token := cfg.GetGitHubToken()
		if token != "env_token" {
			t.Errorf("GetGitHubToken() = %v, want env_token", token)
		}
	})

	t.Run("returns empty string when no token available", func(t *testing.T) {
		ghTokenCache = ""
		ghTokenCached = false
		os.Setenv("PATH", "")

		os.Setenv("GITHUB_TOKEN", "")
		cfg := &Config{}
		token := cfg.GetGitHubToken()
		if token != "" {
			t.Errorf("GetGitHubToken() = %v, want empty string", token)
		}
	})
}
