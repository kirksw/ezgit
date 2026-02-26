package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kirksw/ezgit/internal/github"
)

func TestTabCyclesAllLocalOpenedFilters(t *testing.T) {
	repos := []github.Repo{
		{Name: "foo", FullName: "org/foo"},
		{Name: "bar", FullName: "org/bar"},
	}
	localRepos := map[string]bool{"org/foo": true}

	m := newModel(repos, false, localRepos, false)
	m.openedRepos = map[string]bool{"org/bar": true}

	if m.localOnly {
		t.Fatal("localOnly should start false")
	}
	if m.currentFilterLabel() != "all" {
		t.Fatalf("filter=%q, want all", m.currentFilterLabel())
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
	if m.currentFilterLabel() != "local" {
		t.Fatalf("filter=%q, want local", m.currentFilterLabel())
	}

	updated, _ = m.Update(tab)
	m = updated.(model)

	if m.localOnly {
		t.Fatal("localOnly should be false for opened filter")
	}
	if len(m.repoList.Items()) != 1 {
		t.Fatalf("expected 1 item for opened filter, got %d", len(m.repoList.Items()))
	}
	ri = m.repoList.Items()[0].(repoItem)
	if ri.FullName != "org/bar" {
		t.Fatalf("expected opened repo org/bar, got %s", ri.FullName)
	}
	if m.currentFilterLabel() != "opened" {
		t.Fatalf("filter=%q, want opened", m.currentFilterLabel())
	}

	updated, _ = m.Update(tab)
	m = updated.(model)

	if m.localOnly {
		t.Fatal("localOnly should be false after cycling back to all")
	}
	if len(m.repoList.Items()) != 2 {
		t.Fatalf("expected 2 items when localOnly=false, got %d", len(m.repoList.Items()))
	}
	if m.currentFilterLabel() != "all" {
		t.Fatalf("filter=%q, want all", m.currentFilterLabel())
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

func TestParseWorktreeInlineInput(t *testing.T) {
	name, base := parseWorktreeInlineInput("feature/login:develop")
	if name != "feature/login" {
		t.Fatalf("name=%q, want %q", name, "feature/login")
	}
	if base != "develop" {
		t.Fatalf("base=%q, want %q", base, "develop")
	}

	name, base = parseWorktreeInlineInput("feature/one")
	if name != "feature/one" || base != "" {
		t.Fatalf("got (%q,%q), want (%q,%q)", name, base, "feature/one", "")
	}
}

func TestWorktreeOptionsForRepoHidesRepoRoot(t *testing.T) {
	repo := github.Repo{FullName: "org/foo"}
	m := newModel([]github.Repo{repo}, false, map[string]bool{"org/foo": true}, true)
	m.repoWorktrees[repo.FullName] = []string{"main", "review"}

	options := m.worktreeOptionsForRepo(&repo)
	if len(options) != 3 {
		t.Fatalf("len(options)=%d, want 3", len(options))
	}
	if options[0] != "main" || options[1] != "review" {
		t.Fatalf("unexpected options prefix: %v", options[:2])
	}
	if options[2] != "+ Create new worktree" {
		t.Fatalf("last option=%q, want %q", options[2], "+ Create new worktree")
	}
}

func TestOpenModeViewShowsOpenedWorktreeHighlight(t *testing.T) {
	repo := github.Repo{Name: "foo", FullName: "org/foo"}
	m := newModel([]github.Repo{repo}, false, map[string]bool{"org/foo": true}, true)
	m.repoWorktrees[repo.FullName] = []string{"main"}
	m.openedWorktrees[repo.FullName] = map[string]bool{"main": true}

	view := m.View()
	if !strings.Contains(view, "main") || !strings.Contains(view, "[open]") {
		t.Fatalf("view missing open marker, got: %q", view)
	}
}

func TestMainViewShowsScopeAndTabKeybind(t *testing.T) {
	repo := github.Repo{Name: "foo", FullName: "org/foo"}
	m := newModel([]github.Repo{repo}, false, map[string]bool{"org/foo": true}, true)
	view := m.View()

	if !strings.Contains(view, "Repositories [ALL]") {
		t.Fatalf("view missing scope in pane title, got: %q", view)
	}
	if !strings.Contains(view, "scope: ALL") {
		t.Fatalf("view missing scope in keybind box, got: %q", view)
	}
	if !strings.Contains(view, "tab: scope") {
		t.Fatalf("view missing tab keybind, got: %q", view)
	}
}

func TestRepoListReflectsOpenedReposAfterRefresh(t *testing.T) {
	repo := github.Repo{Name: "foo", FullName: "org/foo"}
	m := newModelWithControls([]github.Repo{repo}, false, map[string]bool{"org/foo": true}, true, true, false)
	m.openedRepos = map[string]bool{"org/foo": true}
	m.repoList.SetItems(m.filterRepos(""))

	items := m.repoList.Items()
	if len(items) != 1 {
		t.Fatalf("len(items)=%d, want 1", len(items))
	}
	ri, ok := items[0].(repoItem)
	if !ok {
		t.Fatalf("unexpected item type %T", items[0])
	}
	if !ri.IsOpen {
		t.Fatal("expected repo to be marked open after refresh")
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
	m := newCloneWorktreeOptionsModel("main", []string{"main"}, true)
	if !m.createDefault {
		t.Fatal("default worktree should start enabled")
	}
	if !m.createReview {
		t.Fatal("review worktree should start enabled")
	}
	if len(m.customWorktree) != 0 {
		t.Fatal("custom worktrees should start empty")
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

func TestFeaturePromptVisibleRangeCentersSelectionWhenPossible(t *testing.T) {
	start, end := featurePromptVisibleRange(20, 10, 6)
	if start != 7 {
		t.Fatalf("start=%d, want 7", start)
	}
	if end != 13 {
		t.Fatalf("end=%d, want 13", end)
	}
}

func TestFeaturePromptVisibleRangeClampsNearEdges(t *testing.T) {
	start, end := featurePromptVisibleRange(20, 0, 6)
	if start != 0 || end != 6 {
		t.Fatalf("near start range=(%d,%d), want (0,6)", start, end)
	}

	start, end = featurePromptVisibleRange(20, 19, 6)
	if start != 14 || end != 20 {
		t.Fatalf("near end range=(%d,%d), want (14,20)", start, end)
	}
}
