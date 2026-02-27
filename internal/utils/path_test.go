package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kirksw/ezgit/internal/github"
)

func TestBuildLocalRepoMapWithWorkersFindsOnlyExistingRepos(t *testing.T) {
	cloneDir := t.TempDir()

	existing := filepath.Join(cloneDir, "acme", "api")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatalf("mkdir existing repo: %v", err)
	}

	repos := []github.Repo{
		{FullName: "acme/api"},
		{FullName: "acme/web"},
		{FullName: "invalid"},
		{FullName: "too/many/parts"},
	}

	local := buildLocalRepoMapWithWorkers(cloneDir, repos, 4)

	if !local["acme/api"] {
		t.Fatalf("expected acme/api to be local")
	}
	if local["acme/web"] {
		t.Fatalf("did not expect acme/web to be local")
	}
	if local["invalid"] || local["too/many/parts"] {
		t.Fatalf("did not expect invalid repo names to be included: %+v", local)
	}
	if len(local) != 1 {
		t.Fatalf("len(local) = %d, want 1", len(local))
	}
}

func TestBuildLocalRepoMapWithWorkersHandlesEmptyInput(t *testing.T) {
	if got := buildLocalRepoMapWithWorkers("", []github.Repo{{FullName: "acme/api"}}, 4); len(got) != 0 {
		t.Fatalf("expected empty map for empty cloneDir, got %d entries", len(got))
	}

	cloneDir := t.TempDir()
	if got := buildLocalRepoMapWithWorkers(cloneDir, nil, 4); len(got) != 0 {
		t.Fatalf("expected empty map for empty repo list, got %d entries", len(got))
	}
}

func TestBuildLocalRepoMapWithWorkersDefaultsWorkerCount(t *testing.T) {
	cloneDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cloneDir, "acme", "api"), 0o755); err != nil {
		t.Fatalf("mkdir existing repo: %v", err)
	}

	repos := []github.Repo{{FullName: "acme/api"}}
	local := buildLocalRepoMapWithWorkers(cloneDir, repos, 0)
	if !local["acme/api"] {
		t.Fatalf("expected acme/api to be local when workers=0")
	}
}
