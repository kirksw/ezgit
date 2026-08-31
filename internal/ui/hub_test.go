package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
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

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(hubModel)
	if !m.worktree {
		t.Fatal("worktree should toggle on in clone mode")
	}
}

func TestHubViewShowsConvertHintOnlyInOpenMode(t *testing.T) {
	m := newHubModel(nil, nil, nil, nil, false)
	m.mode = HubModeOpen
	openView := m.View()
	if !strings.Contains(openView, "ctrl+c: convert") {
		t.Fatal("expected convert hint in open mode")
	}

	m.mode = HubModeClone
	cloneView := m.View()
	if strings.Contains(cloneView, "ctrl+c: convert") {
		t.Fatal("did not expect convert hint in clone mode")
	}
}

func TestHubViewFitsConstrainedWidth(t *testing.T) {
	repos := []github.Repo{{
		Name:        "repository-with-a-name-that-is-too-wide",
		FullName:    "organization/repository-with-a-name-that-is-too-wide",
		Description: "A repository description that must not wrap inside a constrained modal.",
	}}
	m := newHubModel(repos, nil, nil, nil, false)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 32, Height: 20})
	view := updated.(hubModel).View()
	for i, line := range strings.Split(view, "\n") {
		if width := visibleLineWidth(line); width > 32 {
			t.Fatalf("line %d width=%d, want <= 32: %q", i+1, width, line)
		}
	}
	if !strings.Contains(view, "…") {
		t.Fatal("expected constrained content to be truncated")
	}
}

func TestTruncateDisplayWidthHandlesWideRunes(t *testing.T) {
	got := truncateDisplayWidth("界界界", 5)
	if got != "界界…" {
		t.Fatalf("truncateDisplayWidth()=%q, want %q", got, "界界…")
	}
}

func visibleLineWidth(line string) int {
	return lipgloss.Width(line)
}
