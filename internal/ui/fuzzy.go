package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kirksw/ezgit/internal/github"
)

// Action represents what should happen with the selected repo.
type Action int

const (
	ActionClone Action = iota
	ActionOpen
)

type repoDelegate struct{}

func (d repoDelegate) Height() int                             { return 2 }
func (d repoDelegate) Spacing() int                            { return 1 }
func (d repoDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d repoDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	repo, ok := listItem.(repoItem)
	if !ok {
		return
	}

	prefix := ""
	if repo.IsLocal {
		prefix = "* "
	}

	str := fmt.Sprintf("%s%s/%s", prefix, repo.Owner, repo.Name)
	if repo.Description != "" {
		str += fmt.Sprintf("\n  %s", truncateString(repo.Description, 60))
	}

	var style lipgloss.Style
	if index == m.Index() {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	} else {
		style = lipgloss.NewStyle()
	}

	fmt.Fprint(w, style.Render(str))
}

type repoItem struct {
	github.Repo
	Owner   string
	IsLocal bool
}

func (i repoItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s", i.Name, i.FullName, i.Description)
}

// FuzzySearchResult holds the outcome of the fuzzy search TUI.
type FuzzySearchResult struct {
	Repo             *github.Repo
	Worktree         bool
	Action           Action
	SelectedWorktree string
}

type page int

const (
	pageMain page = iota
	pageSettings
	pageWorktrees
)

type model struct {
	repos             []github.Repo
	repoList          list.Model
	textinput         textinput.Model
	quitting          bool
	selected          *github.Repo
	worktree          bool
	localOnly         bool
	localRepos        map[string]bool
	openMode          bool
	currentPage       page
	settingsIndex     int
	worktrees         []string
	worktreeIndex     int
	width             int
	height            int
	lastInput         string
	allowLocalToggle  bool
	allowSettingsPage bool
}

func newModel(repos []github.Repo, worktree bool, localRepos map[string]bool, openMode bool) model {
	return newModelWithControls(repos, worktree, localRepos, openMode, true, true)
}

func newModelWithControls(
	repos []github.Repo,
	worktree bool,
	localRepos map[string]bool,
	openMode bool,
	allowLocalToggle bool,
	allowSettingsPage bool,
) model {
	ti := textinput.New()
	ti.Placeholder = "Search repos..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 80

	l := list.New(nil, repoDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.SetShowPagination(false)
	l.SetWidth(80)
	l.SetHeight(12)

	m := model{
		repos:             repos,
		textinput:         ti,
		repoList:          l,
		worktree:          worktree,
		localRepos:        localRepos,
		openMode:          openMode,
		localOnly:         false,
		currentPage:       pageMain,
		settingsIndex:     0,
		width:             80,
		height:            20,
		allowLocalToggle:  allowLocalToggle,
		allowSettingsPage: allowSettingsPage,
	}

	l.SetItems(m.filterRepos(""))
	m.repoList = l

	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.repoList.SetWidth(msg.Width)
		listHeight := msg.Height - 8
		if listHeight < 4 {
			listHeight = 4
		}
		m.repoList.SetHeight(listHeight)
		m.textinput.Width = msg.Width - 4
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.selected = nil
			m.quitting = true
			return m, tea.Quit
		}

		if m.currentPage == pageMain {
			switch msg.Type {
			case tea.KeyEsc:
				m.quitting = true
				return m, tea.Quit

			case tea.KeyEnter:
				if len(m.repoList.Items()) > 0 {
					if item := m.repoList.SelectedItem(); item != nil {
						if ri, ok := item.(repoItem); ok {
							m.selected = &ri.Repo
							m.quitting = true
							return m, tea.Quit
						}
					}
				}

			case tea.KeyTab:
				if !m.allowLocalToggle {
					return m, nil
				}
				m.localOnly = !m.localOnly
				m.repoList.SetItems(m.filterRepos(m.textinput.Value()))
				m.repoList.ResetSelected()
				return m, nil

			case tea.KeyDown, tea.KeyCtrlN:
				m.repoList.CursorDown()

			case tea.KeyUp, tea.KeyCtrlP:
				m.repoList.CursorUp()

			case tea.KeyCtrlS:
				if !m.allowSettingsPage {
					return m, nil
				}
				m.currentPage = pageSettings
				return m, nil
			}
		} else if m.currentPage == pageWorktrees {
			switch msg.Type {
			case tea.KeyEsc:
				m.currentPage = pageMain
				m.selected = nil
				return m, nil

			case tea.KeyEnter:
				m.quitting = true
				return m, tea.Quit

			case tea.KeyDown, tea.KeyCtrlN:
				m.worktreeIndex = (m.worktreeIndex + 1) % len(m.worktrees)

			case tea.KeyUp, tea.KeyCtrlP:
				m.worktreeIndex = (m.worktreeIndex - 1 + len(m.worktrees)) % len(m.worktrees)
			}
		} else {
			switch msg.Type {
			case tea.KeyEsc:
				m.quitting = true
				return m, tea.Quit

			case tea.KeyCtrlS:
				m.currentPage = pageMain
				return m, nil

			case tea.KeyEnter:
				m.currentPage = pageMain
				return m, nil

			case tea.KeyDown, tea.KeyCtrlN:
				m.settingsIndex = (m.settingsIndex + 1) % 2

			case tea.KeyUp, tea.KeyCtrlP:
				m.settingsIndex = (m.settingsIndex - 1 + 2) % 2

			case tea.KeySpace:
				if m.settingsIndex == 0 {
					m.worktree = !m.worktree
				} else {
					m.openMode = !m.openMode
					m.localOnly = m.openMode
					m.repoList.SetItems(m.filterRepos(m.textinput.Value()))
					m.repoList.ResetSelected()
				}
				return m, nil
			}
		}
	}

	ti, cmd := m.textinput.Update(msg)
	m.textinput = ti

	if current := m.textinput.Value(); current != m.lastInput {
		m.lastInput = current
		m.repoList.SetItems(m.filterRepos(current))
		m.repoList.ResetSelected()
	}

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	switch m.currentPage {
	case pageSettings:
		return m.renderSettingsPage()
	case pageWorktrees:
		return m.renderWorktreesPage()
	default:
		return m.renderMainPage()
	}
}

func (m model) renderMainPage() string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("228")).
		Bold(true)

	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	toggleOnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("120")).
		Bold(true)

	var b strings.Builder

	if m.openMode || m.localOnly {
		b.WriteString(headerStyle.Render("Select a repository to open"))
	} else {
		b.WriteString(headerStyle.Render("Select a repository to clone"))
	}

	if m.allowLocalToggle && m.localOnly {
		b.WriteString("  ")
		b.WriteString(toggleOnStyle.Render("filter: local"))
	}

	b.WriteString("\n\n")
	b.WriteString(m.textinput.View())
	b.WriteString("\n\n")

	if len(m.repoList.Items()) > 0 {
		b.WriteString(m.repoList.View())
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("No repos found"))
	}

	b.WriteString("\n\n")

	var instructions []string
	instructions = append(instructions, "up/down: navigate")
	if m.allowLocalToggle {
		instructions = append(instructions, "tab: toggle local")
	}
	if m.allowSettingsPage {
		instructions = append(instructions, "ctrl+s: settings")
	}
	if m.openMode || m.localOnly {
		instructions = append(instructions, "enter: open")
	} else {
		instructions = append(instructions, "enter: clone")
	}
	instructions = append(instructions, "esc: cancel")
	b.WriteString(instructionStyle.Render(strings.Join(instructions, " | ")))

	return b.String()
}

func (m model) renderSettingsPage() string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("228")).
		Bold(true)

	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	toggleOnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("120")).
		Bold(true)
	toggleOffStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	var b strings.Builder

	b.WriteString(headerStyle.Render("Settings"))
	b.WriteString("\n\n")

	options := []struct {
		name    string
		checked bool
	}{
		{"Worktree mode", m.worktree},
		{"Open mode", m.openMode},
	}

	for i, opt := range options {
		prefix := "  "
		if i == m.settingsIndex {
			prefix = cursorStyle.Render("▶ ")
		}

		checkbox := "[ ]"
		if opt.checked {
			checkbox = toggleOnStyle.Render("[✓]")
		} else {
			checkbox = toggleOffStyle.Render("[ ]")
		}

		b.WriteString(prefix)
		b.WriteString(checkbox)
		b.WriteString(" ")
		b.WriteString(normalStyle.Render(opt.name))
		b.WriteString("\n")
	}

	b.WriteString("\n\n")

	var instructions []string
	instructions = append(instructions, "up/down: navigate")
	instructions = append(instructions, "space: toggle option")
	instructions = append(instructions, "enter/ctrl+s: back to repos")
	instructions = append(instructions, "esc: cancel")
	b.WriteString(instructionStyle.Render(strings.Join(instructions, " | ")))

	return b.String()
}

func (m model) renderWorktreesPage() string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("228")).
		Bold(true)

	repoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("147")).
		Italic(true)

	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	var b strings.Builder

	b.WriteString(headerStyle.Render("Select worktree"))
	b.WriteString("\n")

	if m.selected != nil {
		b.WriteString(repoStyle.Render("  " + m.selected.FullName))
		b.WriteString("\n\n")
	}

	for i, wt := range m.worktrees {
		prefix := "  "
		if i == m.worktreeIndex {
			prefix = cursorStyle.Render("▶ ")
		}

		b.WriteString(prefix)
		b.WriteString(normalStyle.Render(wt))
		b.WriteString("\n")
	}

	b.WriteString("\n\n")

	var instructions []string
	instructions = append(instructions, "up/down: navigate")
	instructions = append(instructions, "enter: confirm")
	instructions = append(instructions, "esc: back")
	b.WriteString(instructionStyle.Render(strings.Join(instructions, " | ")))

	return b.String()
}

func (m model) filterRepos(query string) []list.Item {
	query = strings.ToLower(query)
	var items []list.Item
	for _, repo := range m.repos {
		if m.localOnly && !m.localRepos[repo.FullName] {
			continue
		}
		if query != "" {
			if !strings.Contains(strings.ToLower(repo.Name), query) &&
				!strings.Contains(strings.ToLower(repo.FullName), query) &&
				!strings.Contains(strings.ToLower(repo.Description), query) {
				continue
			}
		}
		owner := ""
		if parts := strings.Split(repo.FullName, "/"); len(parts) == 2 {
			owner = parts[0]
		}
		items = append(items, repoItem{
			Repo:    repo,
			Owner:   owner,
			IsLocal: m.localRepos[repo.FullName],
		})
	}
	return items
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

type featurePromptStep int

const (
	featurePromptStepCreate featurePromptStep = iota
	featurePromptStepBase
	featurePromptStepName
)

type featurePromptModel struct {
	step           featurePromptStep
	options        []string
	optionIndex    int
	baseBranches   []string
	baseIndex      int
	defaultBranch  string
	createFeature  bool
	selectedBase   string
	featureName    string
	featureInput   textinput.Model
	validationHint string
	quitting       bool
	cancelled      bool
}

func normalizeBranchesForFeaturePrompt(branches []string, defaultBranch string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(branches)+1)

	defaultBranch = strings.TrimSpace(defaultBranch)
	if defaultBranch != "" {
		seen[defaultBranch] = struct{}{}
		out = append(out, defaultBranch)
	}

	for _, branch := range branches {
		branch = strings.TrimSpace(strings.TrimPrefix(branch, "origin/"))
		if branch == "" {
			continue
		}
		if _, ok := seen[branch]; ok {
			continue
		}
		seen[branch] = struct{}{}
		out = append(out, branch)
	}

	if len(out) == 0 && defaultBranch != "" {
		out = append(out, defaultBranch)
	}

	return out
}

func newFeaturePromptModel(branches []string, defaultBranch string, askCreate bool) featurePromptModel {
	defaultBranch = strings.TrimSpace(defaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	ti := textinput.New()
	ti.Placeholder = "feature/my-change"
	ti.CharLimit = 128
	ti.Width = 60

	baseBranches := normalizeBranchesForFeaturePrompt(branches, defaultBranch)
	if len(baseBranches) == 0 {
		baseBranches = []string{defaultBranch}
	}
	defaultIndex := 0
	for i, branch := range baseBranches {
		if branch == defaultBranch {
			defaultIndex = i
			break
		}
	}

	step := featurePromptStepBase
	createFeature := true
	options := []string{}
	if askCreate {
		step = featurePromptStepCreate
		createFeature = false
		options = []string{"No custom feature worktree", "Create custom feature worktree"}
	}

	return featurePromptModel{
		step:          step,
		options:       options,
		optionIndex:   0,
		baseBranches:  baseBranches,
		baseIndex:     defaultIndex,
		defaultBranch: defaultBranch,
		createFeature: createFeature,
		selectedBase:  baseBranches[defaultIndex],
		featureInput:  ti,
	}
}

func (m featurePromptModel) Init() tea.Cmd {
	return nil
}

func (m featurePromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit
		}

		switch m.step {
		case featurePromptStepCreate:
			switch msg.Type {
			case tea.KeyUp, tea.KeyCtrlP:
				m.optionIndex = (m.optionIndex - 1 + len(m.options)) % len(m.options)
			case tea.KeyDown, tea.KeyCtrlN:
				m.optionIndex = (m.optionIndex + 1) % len(m.options)
			case tea.KeyEnter:
				if m.optionIndex == 0 {
					m.createFeature = false
					m.quitting = true
					return m, tea.Quit
				}
				m.createFeature = true
				m.selectedBase = m.baseBranches[m.baseIndex]
				m.step = featurePromptStepBase
				return m, nil
			}

		case featurePromptStepBase:
			switch msg.Type {
			case tea.KeyUp, tea.KeyCtrlP:
				m.baseIndex = (m.baseIndex - 1 + len(m.baseBranches)) % len(m.baseBranches)
			case tea.KeyDown, tea.KeyCtrlN:
				m.baseIndex = (m.baseIndex + 1) % len(m.baseBranches)
			case tea.KeyEnter:
				m.selectedBase = m.baseBranches[m.baseIndex]
				m.step = featurePromptStepName
				m.featureInput.Focus()
				return m, nil
			}

		case featurePromptStepName:
			switch msg.Type {
			case tea.KeyEnter:
				featureName := strings.TrimSpace(m.featureInput.Value())
				if featureName == "" {
					m.validationHint = "Feature worktree name cannot be empty"
					return m, nil
				}
				m.featureName = featureName
				m.validationHint = ""
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	if m.step == featurePromptStepName {
		ti, cmd := m.featureInput.Update(msg)
		m.featureInput = ti
		return m, cmd
	}

	return m, nil
}

func (m featurePromptModel) View() string {
	if m.quitting {
		return ""
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("228")).
		Bold(true)
	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)
	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))
	defaultStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("120")).
		Bold(true)
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	var b strings.Builder
	switch m.step {
	case featurePromptStepCreate:
		b.WriteString(headerStyle.Render("Custom feature worktree"))
		b.WriteString("\n\n")
		for i, option := range m.options {
			prefix := "  "
			if i == m.optionIndex {
				prefix = cursorStyle.Render("▶ ")
			}
			b.WriteString(prefix)
			b.WriteString(normalStyle.Render(option))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(instructionStyle.Render("up/down: navigate | enter: confirm | esc: skip custom feature"))

	case featurePromptStepBase:
		b.WriteString(headerStyle.Render("Select base branch for feature worktree"))
		b.WriteString("\n\n")
		for i, branch := range m.baseBranches {
			prefix := "  "
			if i == m.baseIndex {
				prefix = cursorStyle.Render("▶ ")
			}
			label := normalStyle.Render(branch)
			if branch == m.defaultBranch {
				label = defaultStyle.Render(branch + " (default)")
			}
			b.WriteString(prefix)
			b.WriteString(label)
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(instructionStyle.Render("up/down: navigate | enter: select base | esc: skip custom feature"))

	case featurePromptStepName:
		b.WriteString(headerStyle.Render("Feature worktree name"))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render("Base branch: " + m.selectedBase))
		b.WriteString("\n\n")
		b.WriteString(m.featureInput.View())
		b.WriteString("\n")
		if m.validationHint != "" {
			b.WriteString(errorStyle.Render(m.validationHint))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(instructionStyle.Render("type name and enter to confirm | esc: skip custom feature"))
	}

	return b.String()
}

func runFeatureWorktreePromptModel(m featurePromptModel) (featurePromptModel, error) {
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	)
	finalModel, err := p.Run()
	if err != nil {
		return featurePromptModel{}, fmt.Errorf("failed to run feature worktree prompt: %w", err)
	}

	final, ok := finalModel.(featurePromptModel)
	if !ok {
		return featurePromptModel{}, fmt.Errorf("unexpected model type")
	}

	return final, nil
}

func RunFeatureWorktreePrompt(branches []string, defaultBranch string) (create bool, featureBranch string, baseBranch string, err error) {
	m := newFeaturePromptModel(branches, defaultBranch, true)
	final, err := runFeatureWorktreePromptModel(m)
	if err != nil {
		return false, "", "", err
	}

	if final.cancelled || !final.createFeature {
		return false, "", "", nil
	}

	return true, strings.TrimSpace(final.featureName), strings.TrimSpace(final.selectedBase), nil
}

func RunCreateWorktreePrompt(branches []string, defaultBranch string) (featureBranch string, baseBranch string, cancelled bool, err error) {
	m := newFeaturePromptModel(branches, defaultBranch, false)
	final, err := runFeatureWorktreePromptModel(m)
	if err != nil {
		return "", "", false, err
	}
	if final.cancelled {
		return "", "", true, nil
	}
	return strings.TrimSpace(final.featureName), strings.TrimSpace(final.selectedBase), false, nil
}

type cloneWorktreeOption int

const (
	cloneWorktreeOptionDefault cloneWorktreeOption = iota
	cloneWorktreeOptionReview
	cloneWorktreeOptionCustom
)

type cloneWorktreeOptionsModel struct {
	defaultBranch string
	cursor        int
	createDefault bool
	createReview  bool
	addCustom     bool
	cancelled     bool
	quitting      bool
}

func newCloneWorktreeOptionsModel(defaultBranch string) cloneWorktreeOptionsModel {
	defaultBranch = strings.TrimSpace(defaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	return cloneWorktreeOptionsModel{
		defaultBranch: defaultBranch,
		createDefault: true,
		createReview:  true,
		addCustom:     false,
		cursor:        0,
	}
}

func (m cloneWorktreeOptionsModel) Init() tea.Cmd {
	return nil
}

func (m cloneWorktreeOptionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlP:
			m.cursor = (m.cursor - 1 + 3) % 3
		case tea.KeyDown, tea.KeyCtrlN:
			m.cursor = (m.cursor + 1) % 3
		case tea.KeySpace:
			switch cloneWorktreeOption(m.cursor) {
			case cloneWorktreeOptionDefault:
				m.createDefault = !m.createDefault
			case cloneWorktreeOptionReview:
				m.createReview = !m.createReview
			case cloneWorktreeOptionCustom:
				m.addCustom = !m.addCustom
			}
		case tea.KeyEnter:
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m cloneWorktreeOptionsModel) View() string {
	if m.quitting {
		return ""
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("228")).
		Bold(true)
	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)
	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("120")).
		Bold(true)
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	type option struct {
		label   string
		checked bool
	}
	options := []option{
		{label: fmt.Sprintf("Default branch worktree (%s)", m.defaultBranch), checked: m.createDefault},
		{label: fmt.Sprintf("Review worktree (from %s)", m.defaultBranch), checked: m.createReview},
		{label: "Add custom worktree", checked: m.addCustom},
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render("Clone worktree options"))
	b.WriteString("\n\n")

	for i, opt := range options {
		prefix := "  "
		if i == m.cursor {
			prefix = cursorStyle.Render("▶ ")
		}

		box := "[ ]"
		if opt.checked {
			box = selectedStyle.Render("[✓]")
		}

		b.WriteString(prefix)
		b.WriteString(box)
		b.WriteString(" ")
		b.WriteString(normalStyle.Render(opt.label))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(instructionStyle.Render("up/down: navigate | space: toggle | enter: confirm | esc: use defaults"))
	return b.String()
}

func RunCloneWorktreeOptionsPrompt(defaultBranch string) (createDefault bool, createReview bool, addCustom bool, cancelled bool, err error) {
	m := newCloneWorktreeOptionsModel(defaultBranch)
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	)
	finalModel, err := p.Run()
	if err != nil {
		return true, true, false, false, fmt.Errorf("failed to run clone worktree options prompt: %w", err)
	}

	final, ok := finalModel.(cloneWorktreeOptionsModel)
	if !ok {
		return true, true, false, false, fmt.Errorf("unexpected model type")
	}

	if final.cancelled {
		return false, false, false, true, nil
	}

	return final.createDefault, final.createReview, final.addCustom, false, nil
}

// RunFuzzySearch launches the fuzzy repo picker and returns the selected repo
// along with user-selected options like worktree mode and action.
func RunWorktreeSelection(repos []github.Repo, selectedRepo *github.Repo, worktree bool, localRepos map[string]bool, worktrees []string) (*FuzzySearchResult, error) {
	m := newModel(repos, worktree, localRepos, true)
	m.selected = selectedRepo
	m.worktrees = worktrees
	m.worktreeIndex = 0
	m.currentPage = pageWorktrees

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run worktree selection: %w", err)
	}

	model, ok := finalModel.(model)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if model.selected != nil {
		var selectedWorktree string
		if len(model.worktrees) > 0 && model.worktreeIndex < len(model.worktrees) {
			selectedWorktree = model.worktrees[model.worktreeIndex]
		}

		return &FuzzySearchResult{
			Repo:             model.selected,
			Worktree:         model.worktree,
			Action:           ActionOpen,
			SelectedWorktree: selectedWorktree,
		}, nil
	}

	return &FuzzySearchResult{
		Repo:             nil,
		Worktree:         model.worktree,
		Action:           ActionOpen,
		SelectedWorktree: "",
	}, nil
}

func RunFuzzySearch(repos []github.Repo, worktree bool, localRepos map[string]bool, openMode bool) (*FuzzySearchResult, error) {
	p := tea.NewProgram(
		newModel(repos, worktree, localRepos, openMode),
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run fuzzy search: %w", err)
	}

	m, ok := finalModel.(model)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if m.selected != nil {
		action := ActionClone
		if m.openMode || m.localOnly {
			action = ActionOpen
		}

		var selectedWorktree string
		if len(m.worktrees) > 0 && m.worktreeIndex < len(m.worktrees) {
			selectedWorktree = m.worktrees[m.worktreeIndex]
		}

		return &FuzzySearchResult{
			Repo:             m.selected,
			Worktree:         m.worktree,
			Action:           action,
			SelectedWorktree: selectedWorktree,
		}, nil
	}

	return nil, fmt.Errorf("no repository selected")
}

func RunOpenFuzzySearch(repos []github.Repo, localRepos map[string]bool) (*FuzzySearchResult, error) {
	p := tea.NewProgram(
		newModelWithControls(repos, false, localRepos, true, false, false),
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run open fuzzy search: %w", err)
	}

	m, ok := finalModel.(model)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if m.selected != nil {
		return &FuzzySearchResult{
			Repo:             m.selected,
			Worktree:         m.worktree,
			Action:           ActionOpen,
			SelectedWorktree: "",
		}, nil
	}

	return nil, fmt.Errorf("no repository selected")
}
