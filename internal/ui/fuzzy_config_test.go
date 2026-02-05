package ui

import (
	"testing"

	"github.com/kirksw/ezgit/internal/github"
)

func TestNewModelWithOpenMode(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
	}
	localRepos := map[string]bool{"org/foo": true}

	m := newModel(repos, false, localRepos, false)

	if m.openMode {
		t.Fatal("openMode should start false when passed false")
	}

	if m.localOnly {
		t.Fatal("localOnly should be false by default")
	}

	m2 := newModel(repos, false, localRepos, true)

	if !m2.openMode {
		t.Fatal("openMode should start true when passed true")
	}

	if m2.localOnly {
		t.Fatal("localOnly should be false by default even when openMode is true")
	}
}
