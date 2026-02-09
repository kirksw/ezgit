package cmd

import (
	"testing"
	"time"

	"github.com/kirksw/ezgit/internal/github"
)

func TestMergeReposByFullName(t *testing.T) {
	oldest := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	middle := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newest := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	existing := []github.Repo{
		{FullName: "acme/old", CreatedAt: oldest, Description: "old"},
		{FullName: "acme/shared", CreatedAt: middle, Description: "existing"},
	}
	incoming := []github.Repo{
		{FullName: "acme/new", CreatedAt: newest, Description: "new"},
		{FullName: "acme/shared", CreatedAt: middle, Description: "updated"},
	}

	merged, added := mergeReposByFullName(existing, incoming)
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if len(merged) != 3 {
		t.Fatalf("len(merged) = %d, want 3", len(merged))
	}
	if merged[0].FullName != "acme/new" {
		t.Fatalf("merged[0] = %s, want acme/new", merged[0].FullName)
	}

	var shared github.Repo
	for _, repo := range merged {
		if repo.FullName == "acme/shared" {
			shared = repo
			break
		}
	}
	if shared.Description != "updated" {
		t.Fatalf("shared.Description = %q, want %q", shared.Description, "updated")
	}
}
