package cmd

import (
	"testing"

	"github.com/kirksw/ezgit/internal/github"
)

func TestSortedRepoNamesFiltersLocalRepos(t *testing.T) {
	repos := []github.Repo{
		{FullName: "acme/api"},
		{FullName: "acme/web"},
		{FullName: ""},
	}
	got := sortedRepoNames(repos, map[string]bool{"acme/web": true}, true)
	if len(got) != 1 || got[0] != "acme/web" {
		t.Fatalf("sortedRepoNames local = %v, want [acme/web]", got)
	}
}

func TestAgentCommandsAreRegistered(t *testing.T) {
	for _, path := range [][]string{{"list", "orgs"}, {"list", "repos"}, {"clone"}, {"add"}, {"open"}} {
		cmd := rootCmd
		for _, name := range path {
			next, _, err := cmd.Find([]string{name})
			if err != nil || next == cmd || next.Name() != name {
				t.Fatalf("command %v is not registered", path)
			}
			cmd = next
		}
	}
}
