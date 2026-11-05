package cmd

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kirksw/ezgit/internal/config"
)

func runSafetyGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

// newSafetyEnv isolates HOME and git config, and serves git@github.com:acme/*
// from local bare repositories so no test can reach the network.
func newSafetyEnv(t *testing.T) (*config.Config, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	remotes := filepath.Join(root, "remotes")
	cloneDir := filepath.Join(root, "clones")
	for _, dir := range []string{home, filepath.Join(remotes, "acme"), cloneDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll(%s) failed: %v", dir, err)
		}
	}

	gitConfig := filepath.Join(home, ".gitconfig")
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_SSH_COMMAND", "false")
	runSafetyGit(t, root, "config", "--global", "user.email", "test@example.com")
	runSafetyGit(t, root, "config", "--global", "user.name", "test")
	runSafetyGit(t, root, "config", "--global", "url.file://"+remotes+"/.insteadOf", "git@github.com:")

	source := filepath.Join(root, "source")
	runSafetyGit(t, root, "init", "-b", "main", source)
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("failed to write README.md: %v", err)
	}
	runSafetyGit(t, source, "add", "README.md")
	runSafetyGit(t, source, "commit", "-m", "initial commit")
	runSafetyGit(t, source, "branch", "develop")
	runSafetyGit(t, root, "clone", "--bare", source, filepath.Join(remotes, "acme", "repo.git"))

	prevLoader, prevZoxide := loadRepoDefaultBranches, runZoxideAdd
	loadRepoDefaultBranches = func() map[string]string { return map[string]string{} }
	runZoxideAdd = func(string) error { return nil }
	resetDefaultBranchLookupCache()
	t.Cleanup(func() {
		loadRepoDefaultBranches, runZoxideAdd = prevLoader, prevZoxide
		resetDefaultBranchLookupCache()
	})

	return &config.Config{Git: config.GitConfig{CloneDir: cloneDir, OpenCommand: "true"}}, filepath.Join(cloneDir, "acme", "repo")
}

func cloneSafetyRepo(t *testing.T, cfg *config.Config, repoPath string) {
	t.Helper()
	if err := runDirectClone(cfg, "acme/repo", "main", 0); err != nil {
		t.Fatalf("runDirectClone() error = %v", err)
	}
	runSafetyGit(t, repoPath, "status")
}

func setConvertFlags(t *testing.T, selected []string, none bool) {
	t.Helper()
	prevWorktrees, prevAll, prevNone := worktrees, allWorktrees, noWorktrees
	worktrees, allWorktrees, noWorktrees = selected, false, none
	t.Cleanup(func() {
		worktrees, allWorktrees, noWorktrees = prevWorktrees, prevAll, prevNone
	})
}

func useDevNullStdin(t *testing.T) {
	t.Helper()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("failed to open %s: %v", os.DevNull, err)
	}
	prevStdin := os.Stdin
	os.Stdin = devNull
	t.Cleanup(func() {
		os.Stdin = prevStdin
		devNull.Close()
	})
}

func TestCloneDoesNotRequireDefaultSSHKey(t *testing.T) {
	cfg, repoPath := newSafetyEnv(t)
	useDevNullStdin(t)

	cloneSafetyRepo(t, cfg, repoPath)

	cmd := exec.Command("git", "config", "--local", "core.sshCommand")
	cmd.Dir = repoPath
	if output, err := cmd.Output(); err == nil {
		t.Fatalf("core.sshCommand = %q, want unset without --key-path", output)
	}
}

func TestCloneSkipsKeyValidationWhenRepoAlreadyExists(t *testing.T) {
	cfg, repoPath := newSafetyEnv(t)
	useDevNullStdin(t)
	cloneSafetyRepo(t, cfg, repoPath)

	prevKeyPath := keyPath
	keyPath = filepath.Join(t.TempDir(), "missing-key")
	t.Cleanup(func() { keyPath = prevKeyPath })

	if err := runDirectClone(cfg, "acme/repo", "main", 0); err != nil {
		t.Fatalf("runDirectClone() on existing repo error = %v", err)
	}
}

func TestCloneRejectsMissingExplicitSSHKey(t *testing.T) {
	cfg, repoPath := newSafetyEnv(t)
	useDevNullStdin(t)

	prevKeyPath := keyPath
	keyPath = filepath.Join(t.TempDir(), "missing-key")
	t.Cleanup(func() { keyPath = prevKeyPath })

	err := runDirectClone(cfg, "acme/repo", "main", 0)
	if err == nil || !strings.Contains(err.Error(), "SSH key validation failed") {
		t.Fatalf("runDirectClone() error = %v, want SSH key validation failure", err)
	}
	if _, statErr := os.Stat(repoPath); !os.IsNotExist(statErr) {
		t.Fatalf("destination should not be created, err = %v", statErr)
	}
}

func TestOpenWorktreeDoesNotConvertRegularClone(t *testing.T) {
	cfg, repoPath := newSafetyEnv(t)
	useDevNullStdin(t)
	cloneSafetyRepo(t, cfg, repoPath)
	untracked := filepath.Join(repoPath, "untracked.txt")
	if err := os.WriteFile(untracked, []byte("precious\n"), 0644); err != nil {
		t.Fatalf("failed to write untracked file: %v", err)
	}

	err := ensureOpenWorktree(cfg, "acme/repo", "feature")
	if err == nil || !strings.Contains(err.Error(), "ezgit convert") {
		t.Fatalf("ensureOpenWorktree() error = %v, want guidance to run convert", err)
	}
	if _, statErr := os.Stat(untracked); statErr != nil {
		t.Fatalf("untracked file should survive: %v", statErr)
	}
	if got := runSafetyGit(t, repoPath, "rev-parse", "--is-bare-repository"); got != "false" {
		t.Fatalf("is-bare-repository = %q, want false", got)
	}
}

func TestConvertRefusesDirtyRepoWithoutChangingIt(t *testing.T) {
	cfg, repoPath := newSafetyEnv(t)
	useDevNullStdin(t)
	cloneSafetyRepo(t, cfg, repoPath)
	setConvertFlags(t, nil, false)
	untracked := filepath.Join(repoPath, "untracked.txt")
	if err := os.WriteFile(untracked, []byte("precious\n"), 0644); err != nil {
		t.Fatalf("failed to write untracked file: %v", err)
	}

	if err := runConvertPath(repoPath, "main"); err == nil {
		t.Fatal("runConvertPath() error = nil, want refusal")
	}
	if _, err := os.Stat(untracked); err != nil {
		t.Fatalf("untracked file should survive: %v", err)
	}
}

func TestConvertValidatesBranchesBeforeChangingRepo(t *testing.T) {
	cfg, repoPath := newSafetyEnv(t)
	useDevNullStdin(t)
	cloneSafetyRepo(t, cfg, repoPath)
	setConvertFlags(t, []string{"no-such-branch"}, false)

	err := runConvertPath(repoPath, "main")
	if err == nil || !strings.Contains(err.Error(), "branch not found") {
		t.Fatalf("runConvertPath() error = %v, want branch validation failure", err)
	}
	if got := runSafetyGit(t, repoPath, "rev-parse", "--is-bare-repository"); got != "false" {
		t.Fatalf("is-bare-repository = %q, want false", got)
	}
	if _, err := os.Stat(filepath.Join(repoPath, "README.md")); err != nil {
		t.Fatalf("working tree should be untouched: %v", err)
	}
}

func TestConvertCreatesWorktreeForRemoteOnlyBranchAndKeepsOrigin(t *testing.T) {
	cfg, repoPath := newSafetyEnv(t)
	useDevNullStdin(t)
	cloneSafetyRepo(t, cfg, repoPath)
	origin := runSafetyGit(t, repoPath, "remote", "get-url", "origin")
	setConvertFlags(t, []string{"develop"}, false)

	if err := runConvertPath(repoPath, "main"); err != nil {
		t.Fatalf("runConvertPath() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, "develop", "README.md")); err != nil {
		t.Fatalf("develop worktree missing: %v", err)
	}
	if got := runSafetyGit(t, filepath.Join(repoPath, ".git"), "remote", "get-url", "origin"); got != origin {
		t.Fatalf("origin = %q, want %q", got, origin)
	}
}

func TestConvertRefusesExistingWorktreeLayout(t *testing.T) {
	cfg, repoPath := newSafetyEnv(t)
	useDevNullStdin(t)
	cloneSafetyRepo(t, cfg, repoPath)
	setConvertFlags(t, nil, false)
	if err := runConvertPath(repoPath, "main"); err != nil {
		t.Fatalf("first runConvertPath() error = %v", err)
	}
	wip := filepath.Join(repoPath, "main", "wip.txt")
	if err := os.WriteFile(wip, []byte("wip\n"), 0644); err != nil {
		t.Fatalf("failed to write wip file: %v", err)
	}

	err := runConvertPath(repoPath, "main")
	if err == nil || !strings.Contains(err.Error(), "already uses worktree layout") {
		t.Fatalf("second runConvertPath() error = %v, want refusal", err)
	}
	if _, statErr := os.Stat(wip); statErr != nil {
		t.Fatalf("worktree file should survive: %v", statErr)
	}
}

func TestIsInteractiveStdinIsFalseForDevNull(t *testing.T) {
	useDevNullStdin(t)
	if isInteractiveStdin() {
		t.Fatal("isInteractiveStdin() = true for /dev/null, want false")
	}
}

func TestPromptExistingRegularRepoAction(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    existingRepoAction
		wantEOF bool
	}{
		{name: "open", input: "o\n", want: existingRepoActionOpen},
		{name: "convert", input: "convert\n", want: existingRepoActionConvert},
		{name: "cancel", input: "x\n", want: existingRepoActionCancel},
		{name: "answer without newline", input: "c", want: existingRepoActionConvert},
		{name: "retries invalid answer", input: "nope\no\n", want: existingRepoActionOpen},
		{name: "empty input stops", input: "", want: existingRepoActionCancel, wantEOF: true},
		{name: "invalid answer then eof stops", input: "nope\n", want: existingRepoActionCancel, wantEOF: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := promptExistingRegularRepoAction(strings.NewReader(tt.input), io.Discard, "/repo")
			if tt.wantEOF != errors.Is(err, io.EOF) {
				t.Fatalf("error = %v, wantEOF %v", err, tt.wantEOF)
			}
			if !tt.wantEOF && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("action = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepoInputValidationRejectsPathEscapes(t *testing.T) {
	tests := []struct {
		input  string
		wantOK bool
	}{
		{input: "acme/repo", wantOK: true},
		{input: "acme/.github", wantOK: true},
		{input: "acme/repo.js", wantOK: true},
		{input: "acme_corp/repo_1", wantOK: true},
		{input: "git@github.com:acme/repo.git", wantOK: true},
		{input: "https://github.com/acme/repo", wantOK: true},
		{input: "../..", wantOK: false},
		{input: "../repo", wantOK: false},
		{input: "acme/..", wantOK: false},
		{input: "acme/.", wantOK: false},
		{input: "-acme/repo", wantOK: false},
		{input: "acme/-repo", wantOK: false},
		{input: "acme/re po", wantOK: false},
		{input: "../../outside/x", wantOK: false},
		{input: "https://github.com/../repo", wantOK: false},
		{input: "git@github.com:../repo.git", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if err := validateRepoInput(tt.input); (err == nil) != tt.wantOK {
				t.Fatalf("validateRepoInput(%q) error = %v, wantOK %v", tt.input, err, tt.wantOK)
			}
		})
	}
}

func TestOpenWorktreeRejectsPathEscapingName(t *testing.T) {
	cfg, repoPath := newSafetyEnv(t)
	useDevNullStdin(t)
	prevWorktree := worktree
	t.Cleanup(func() { worktree = prevWorktree })
	worktree = true
	if err := runDirectClone(cfg, "acme/repo", "main", 0); err != nil {
		t.Fatalf("runDirectClone() error = %v", err)
	}
	worktree = false

	if err := ensureOpenWorktree(cfg, "acme/repo", "../.."); err == nil {
		t.Fatal("ensureOpenWorktree(../..) error = nil, want rejection")
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		t.Fatalf("repo should be untouched: %v", err)
	}
}

func TestCacheCommandsRejectPathEscapingOrg(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	victim := filepath.Join(home, "victim.json")
	if err := os.WriteFile(victim, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write victim file: %v", err)
	}

	if err := runInvalidateCache(nil, []string{"../../victim"}); err == nil {
		t.Fatal("runInvalidateCache() error = nil, want rejection")
	}
	if err := runRefreshCache(nil, []string{"../../victim"}); err == nil || !strings.Contains(err.Error(), "invalid organization name") {
		t.Fatalf("runRefreshCache() error = %v, want invalid organization name", err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("file outside the cache should survive: %v", err)
	}
}
