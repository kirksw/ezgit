package ui

import (
	"strings"
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

func TestOpenOnlyModelDisablesLocalToggleAndSettings(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
	}
	localRepos := map[string]bool{"org/foo": true}

	m := newModelWithControls(repos, false, localRepos, true, false, false)

	tab := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(tab)
	m = updated.(model)
	if m.localOnly {
		t.Fatal("localOnly should remain false when local toggle is disabled")
	}

	ctrlS := tea.KeyMsg{Type: tea.KeyCtrlS}
	updated, _ = m.Update(ctrlS)
	m = updated.(model)
	if m.currentPage != pageMain {
		t.Fatal("currentPage should remain main when settings are disabled")
	}
}

func TestOpenOnlyViewHidesLocalToggleAndSettingsHints(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
	}
	localRepos := map[string]bool{"org/foo": true}

	m := newModelWithControls(repos, false, localRepos, true, false, false)
	view := m.View()

	if strings.Contains(view, "tab: toggle local") {
		t.Fatal("view should not show local toggle hint for open-only mode")
	}
	if strings.Contains(view, "ctrl+s: settings") {
		t.Fatal("view should not show settings hint for open-only mode")
	}
}

func TestNormalizeBranchesForFeaturePrompt(t *testing.T) {
	got := normalizeBranchesForFeaturePrompt(
		[]string{"origin/main", "develop", "origin/develop", "release", "main"},
		"main",
	)
	want := []string{"main", "develop", "release"}

	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestNewFeaturePromptModelDefaultBranchSelected(t *testing.T) {
	m := newFeaturePromptModel([]string{"develop", "main"}, "main", true)
	if len(m.baseBranches) == 0 {
		t.Fatal("expected base branches")
	}
	if m.baseBranches[m.baseIndex] != "main" {
		t.Fatalf("selected base branch=%q, want %q", m.baseBranches[m.baseIndex], "main")
	}
}

func TestNewFeaturePromptModelWithoutCreateStepStartsAtBaseSelection(t *testing.T) {
	m := newFeaturePromptModel([]string{"develop", "main"}, "main", false)
	if m.step != featurePromptStepBase {
		t.Fatalf("step=%v, want %v", m.step, featurePromptStepBase)
	}
	if !m.createFeature {
		t.Fatal("createFeature should be true when create step is skipped")
	}
	if m.selectedBase != "main" {
		t.Fatalf("selectedBase=%q, want %q", m.selectedBase, "main")
	}
}

func TestNewCloneWorktreeOptionsModelDefaults(t *testing.T) {
	m := newCloneWorktreeOptionsModel("main")
	if !m.createDefault {
		t.Fatal("default worktree should start enabled")
	}
	if !m.createReview {
		t.Fatal("review worktree should start enabled")
	}
	if m.addCustom {
		t.Fatal("custom worktree should start disabled")
	}
}

func TestCtrlCCancelsFromWorktreePage(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
	}
	localRepos := map[string]bool{"org/foo": true}

	selected := repos[0]
	m := newModel(repos, false, localRepos, true)
	m.currentPage = pageWorktrees
	m.selected = &selected
	m.worktrees = []string{"main", "review"}
	m.worktreeIndex = 0

	ctrlC := tea.KeyMsg{Type: tea.KeyCtrlC}
	updated, _ := m.Update(ctrlC)
	m = updated.(model)

	if !m.quitting {
		t.Fatal("model should quit on ctrl+c")
	}
	if m.selected != nil {
		t.Fatal("selected repo should be cleared on ctrl+c cancel")
	}
}
