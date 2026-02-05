package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kirksw/ezgit/internal/github"
)

func TestTabTogglesLocalOnly(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
		{Name: "bar", FullName: "org/bar"},
	}
	localRepos := map[string]bool{"org/foo": true}

	m := newModel(repos, false, localRepos, false)

	if m.localOnly {
		t.Fatal("localOnly should start false")
	}

	tab := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(tab)
	m = updated.(model)

	if !m.localOnly {
		t.Fatal("localOnly should be true after Tab")
	}

	items := m.repoList.Items()
	if len(items) != 1 {
		t.Fatalf("expected 1 item when localOnly=true, got %d", len(items))
	}
	ri := items[0].(repoItem)
	if ri.FullName != "org/foo" {
		t.Fatalf("expected org/foo, got %s", ri.FullName)
	}

	updated, _ = m.Update(tab)
	m = updated.(model)

	if m.localOnly {
		t.Fatal("localOnly should be false after second Tab")
	}
	if len(m.repoList.Items()) != 2 {
		t.Fatalf("expected 2 items when localOnly=false, got %d", len(m.repoList.Items()))
	}
}

func TestCtrlSTogglesPages(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
	}
	localRepos := map[string]bool{"org/foo": true}

	m := newModel(repos, false, localRepos, false)

	if m.currentPage != pageMain {
		t.Fatal("should start on main page")
	}

	ctrlS := tea.KeyMsg{Type: tea.KeyCtrlS}
	updated, _ := m.Update(ctrlS)
	m = updated.(model)

	if m.currentPage != pageSettings {
		t.Fatal("should be on settings page after ctrl+s")
	}

	updated, _ = m.Update(ctrlS)
	m = updated.(model)

	if m.currentPage != pageMain {
		t.Fatal("should be back on main page after ctrl+s")
	}
}

func TestSettingsPageTogglesWorktree(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
	}
	localRepos := map[string]bool{"org/foo": true}

	m := newModel(repos, false, localRepos, false)

	ctrlS := tea.KeyMsg{Type: tea.KeyCtrlS}
	updated, _ := m.Update(ctrlS)
	m = updated.(model)

	if m.settingsIndex != 0 {
		t.Fatalf("settingsIndex should be 0, got %d", m.settingsIndex)
	}

	space := tea.KeyMsg{Type: tea.KeySpace}
	updated, _ = m.Update(space)
	m = updated.(model)

	if !m.worktree {
		t.Fatal("worktree should be true after space")
	}

	updated, _ = m.Update(space)
	m = updated.(model)

	if m.worktree {
		t.Fatal("worktree should be false after second space")
	}
}

func TestSettingsPageTogglesOpenMode(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
	}
	localRepos := map[string]bool{"org/foo": true}

	m := newModel(repos, false, localRepos, false)

	ctrlS := tea.KeyMsg{Type: tea.KeyCtrlS}
	updated, _ := m.Update(ctrlS)
	m = updated.(model)

	down := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ = m.Update(down)
	m = updated.(model)

	if m.settingsIndex != 1 {
		t.Fatalf("settingsIndex should be 1, got %d", m.settingsIndex)
	}

	space := tea.KeyMsg{Type: tea.KeySpace}
	updated, _ = m.Update(space)
	m = updated.(model)

	if !m.openMode {
		t.Fatal("openMode should be true after space")
	}

	if !m.localOnly {
		t.Fatal("localOnly should be true when openMode is true")
	}

	updated, _ = m.Update(space)
	m = updated.(model)

	if m.openMode {
		t.Fatal("openMode should be false after second space")
	}
}

func TestSettingsPageNavigation(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
	}
	localRepos := map[string]bool{"org/foo": true}

	m := newModel(repos, false, localRepos, false)

	ctrlS := tea.KeyMsg{Type: tea.KeyCtrlS}
	updated, _ := m.Update(ctrlS)
	m = updated.(model)

	if m.settingsIndex != 0 {
		t.Fatalf("settingsIndex should start at 0, got %d", m.settingsIndex)
	}

	down := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ = m.Update(down)
	m = updated.(model)

	if m.settingsIndex != 1 {
		t.Fatalf("settingsIndex should be 1 after down, got %d", m.settingsIndex)
	}

	updated, _ = m.Update(down)
	m = updated.(model)

	if m.settingsIndex != 0 {
		t.Fatalf("settingsIndex should wrap to 0, got %d", m.settingsIndex)
	}

	up := tea.KeyMsg{Type: tea.KeyUp}
	updated, _ = m.Update(up)
	m = updated.(model)

	if m.settingsIndex != 1 {
		t.Fatalf("settingsIndex should wrap to 1 after up, got %d", m.settingsIndex)
	}
}
