package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kirksw/ezgit/internal/github"
)

func TestHubModeSwitchReplacesItems(t *testing.T) {
	allRepos := []github.Repo{
		{Name: "alpha", FullName: "org/alpha"},
		{Name: "beta", FullName: "org/beta"},
	}
	openRepos := []github.Repo{
		{Name: "alpha", FullName: "org/alpha"},
	}
	local := map[string]bool{
		"org/alpha": true,
	}

	m := newHubModel(allRepos, openRepos, local, []string{"dev"}, false)
	if len(m.list.Items()) != 2 {
		t.Fatalf("clone items=%d, want 2", len(m.list.Items()))
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(hubModel)
	if m.mode != HubModeOpen {
		t.Fatalf("mode=%v, want open", m.mode)
	}
	if len(m.list.Items()) != 1 {
		t.Fatalf("open items=%d, want 1", len(m.list.Items()))
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(hubModel)
	if m.mode != HubModeConnect {
		t.Fatalf("mode=%v, want connect", m.mode)
	}
	if len(m.list.Items()) != 1 {
		t.Fatalf("connect items=%d, want 1", len(m.list.Items()))
	}
}

func TestHubOpenModeConvertShortcut(t *testing.T) {
	openRepos := []github.Repo{
		{Name: "alpha", FullName: "org/alpha"},
	}

	m := newHubModel(nil, openRepos, map[string]bool{"org/alpha": true}, nil, false)
	m.mode = HubModeOpen
	m.refreshItems()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = updated.(hubModel)
	if m.action != hubActionConvert {
		t.Fatalf("action=%v, want convert", m.action)
	}
	if m.selectedRepo == nil || m.selectedRepo.FullName != "org/alpha" {
		t.Fatal("expected selected open repo for convert action")
	}
}

func TestHubCloneWorktreeToggle(t *testing.T) {
	m := newHubModel(nil, nil, nil, nil, false)
	if m.worktree {
		t.Fatal("worktree should start false")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = updated.(hubModel)
	if !m.worktree {
		t.Fatal("worktree should toggle on in clone mode")
	}
}

func TestHubViewShowsConvertHintOnlyInOpenMode(t *testing.T) {
	m := newHubModel(nil, nil, nil, nil, false)
	m.mode = HubModeOpen
	openView := m.View()
	if !strings.Contains(openView, "c: convert") {
		t.Fatal("expected convert hint in open mode")
	}

	m.mode = HubModeClone
	cloneView := m.View()
	if strings.Contains(cloneView, "c: convert") {
		t.Fatal("did not expect convert hint in clone mode")
	}
}
