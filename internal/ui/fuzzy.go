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

	localBadgeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("119")).
		Background(lipgloss.Color("236")).
		Padding(0, 1)
	openBadgeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("81")).
		Background(lipgloss.Color("236")).
		Padding(0, 1)

	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	if index == m.Index() {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
		descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	}

	badges := make([]string, 0, 2)
	if repo.IsLocal {
		badges = append(badges, localBadgeStyle.Render("[local]"))
	}
	if repo.IsOpen {
		badges = append(badges, openBadgeStyle.Render("[open]"))
	}

	line := nameStyle.Render(fmt.Sprintf("%s/%s", repo.Owner, repo.Name))
	if len(badges) > 0 {
		line = strings.Join(badges, " ") + " " + line
	}

	if repo.Description != "" {
		line += fmt.Sprintf("\n  %s", descStyle.Render(truncateString(repo.Description, 60)))
	}

	fmt.Fprint(w, line)
}

type repoItem struct {
	github.Repo
	Owner   string
	IsLocal bool
	IsOpen  bool
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
	CreateWorktree   bool
	WorktreeBase     string
}

type page int

const (
	pageMain page = iota
	pageSettings
	pageWorktrees
)

type model struct {
	repos              []github.Repo
	repoList           list.Model
	textinput          textinput.Model
	quitting           bool
	selected           *github.Repo
	worktree           bool
	localOnly          bool
	filterIndex        int
	localRepos         map[string]bool
	openedRepos        map[string]bool
	openedWorktrees    map[string]map[string]bool
	openMode           bool
	repoWorktrees      map[string][]string
	worktreeSelection  map[string]int
	focusWorktreePane  bool
	creatingWorktree   bool
	worktreeInput      textinput.Model
	worktreeInputHint  string
	selectedWorktree   string
	createWorktree     bool
	createWorktreeBase string
	currentPage        page
	settingsIndex      int
	worktrees          []string
	worktreeIndex      int
	width              int
	height             int
	lastInput          string
	allowLocalToggle   bool
	allowSettingsPage  bool
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
		openedRepos:       map[string]bool{},
		openedWorktrees:   map[string]map[string]bool{},
		repoWorktrees:     map[string][]string{},
		worktreeSelection: map[string]int{},
		openMode:          openMode,
		localOnly:         false,
		filterIndex:       0,
		currentPage:       pageMain,
		settingsIndex:     0,
		width:             80,
		height:            20,
		allowLocalToggle:  allowLocalToggle,
		allowSettingsPage: allowSettingsPage,
	}

	wtInput := textinput.New()
	wtInput.Placeholder = "new-worktree[:base]"
	wtInput.CharLimit = 256
	wtInput.Width = 40
	m.worktreeInput = wtInput

	l.SetItems(m.filterRepos(""))
	m.repoList = l

	return m
}

func (m model) currentFilterLabel() string {
	switch m.filterIndex {
	case 1:
		return "local"
	case 2:
		return "opened"
	default:
		return "all"
	}
}

func (m *model) cycleFilter() {
	m.filterIndex = (m.filterIndex + 1) % 3
	m.localOnly = m.filterIndex == 1
	m.repoList.SetItems(m.filterRepos(m.textinput.Value()))
	m.repoList.ResetSelected()
}

func (m model) selectedRepoFromList() *github.Repo {
	if item := m.repoList.SelectedItem(); item != nil {
		if ri, ok := item.(repoItem); ok {
			repo := ri.Repo
			return &repo
		}
	}
	return nil
}

func (m model) worktreeOptionsForRepo(repo *github.Repo) []string {
	options := make([]string, 0)
	if repo == nil {
		options = append(options, "+ Create new worktree")
		return options
	}
	for _, wt := range m.repoWorktrees[repo.FullName] {
		trimmed := strings.TrimSpace(wt)
		if trimmed == "" {
			continue
		}
		options = append(options, trimmed)
	}
	options = append(options, "+ Create new worktree")
	return options
}

func parseWorktreeInlineInput(input string) (name string, base string) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", ""
	}
	parts := strings.SplitN(value, ":", 2)
	name = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		base = strings.TrimSpace(parts[1])
	}
	return name, base
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listWidth := msg.Width
		if m.openMode {
			listWidth = msg.Width / 2
			if listWidth < 40 {
				listWidth = msg.Width
			}
		}
		m.repoList.SetWidth(listWidth)
		reservedLines := 14
		if m.openMode {
			reservedLines = 16
		}
		listHeight := msg.Height - reservedLines
		if listHeight < 4 {
			listHeight = 4
		}
		m.repoList.SetHeight(listHeight)
		m.textinput.Width = msg.Width - 4
		m.worktreeInput.Width = listWidth - 4
		if m.worktreeInput.Width < 20 {
			m.worktreeInput.Width = 20
		}
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.selected = nil
			m.quitting = true
			return m, tea.Quit
		}

		if m.currentPage == pageMain {
			if m.creatingWorktree {
				switch msg.Type {
				case tea.KeyEsc:
					m.creatingWorktree = false
					m.worktreeInputHint = ""
					m.worktreeInput.Blur()
					return m, nil
				case tea.KeyEnter:
					name, base := parseWorktreeInlineInput(m.worktreeInput.Value())
					if name == "" {
						m.worktreeInputHint = "Enter worktree name or name:base"
						return m, nil
					}
					repo := m.selectedRepoFromList()
					if repo == nil {
						m.worktreeInputHint = "Select a repository first"
						return m, nil
					}
					m.selected = repo
					m.selectedWorktree = name
					m.createWorktree = true
					m.createWorktreeBase = base
					m.quitting = true
					return m, tea.Quit
				}
			}

			switch msg.Type {
			case tea.KeyEsc:
				m.quitting = true
				return m, tea.Quit

			case tea.KeyEnter:
				if m.openMode && m.focusWorktreePane {
					repo := m.selectedRepoFromList()
					if repo == nil {
						return m, nil
					}
					options := m.worktreeOptionsForRepo(repo)
					idx := m.worktreeSelection[repo.FullName]
					if idx < 0 || idx >= len(options) {
						idx = 0
					}
					selected := options[idx]
					if selected == "+ Create new worktree" {
						m.creatingWorktree = true
						m.worktreeInputHint = ""
						m.worktreeInput.SetValue("")
						m.worktreeInput.Focus()
						return m, nil
					}

					m.selected = repo
					m.selectedWorktree = selected
					m.quitting = true
					return m, tea.Quit
				}

				if len(m.repoList.Items()) > 0 {
					if item := m.repoList.SelectedItem(); item != nil {
						if ri, ok := item.(repoItem); ok {
							m.selected = &ri.Repo
							m.selectedWorktree = ""
							m.quitting = true
							return m, tea.Quit
						}
					}
				}

			case tea.KeyTab:
				if !m.allowLocalToggle {
					return m, nil
				}
				m.cycleFilter()
				return m, nil

			case tea.KeyDown, tea.KeyCtrlN:
				if m.openMode && m.focusWorktreePane {
					repo := m.selectedRepoFromList()
					if repo != nil {
						options := m.worktreeOptionsForRepo(repo)
						idx := m.worktreeSelection[repo.FullName]
						idx = (idx + 1) % len(options)
						m.worktreeSelection[repo.FullName] = idx
					}
				} else {
					m.repoList.CursorDown()
				}

			case tea.KeyUp, tea.KeyCtrlP:
				if m.openMode && m.focusWorktreePane {
					repo := m.selectedRepoFromList()
					if repo != nil {
						options := m.worktreeOptionsForRepo(repo)
						idx := m.worktreeSelection[repo.FullName]
						idx = (idx - 1 + len(options)) % len(options)
						m.worktreeSelection[repo.FullName] = idx
					}
				} else {
					m.repoList.CursorUp()
				}

			case tea.KeyRight:
				if m.openMode {
					m.focusWorktreePane = true
					return m, nil
				}

			case tea.KeyLeft:
				if m.openMode {
					m.focusWorktreePane = false
					return m, nil
				}

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

	if m.currentPage == pageMain && m.creatingWorktree {
		input, cmd := m.worktreeInput.Update(msg)
		m.worktreeInput = input
		return m, cmd
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
		Foreground(lipgloss.Color("229")).
		Bold(true)

	searchBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	paneStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	focusedPaneStyle := paneStyle.Copy().
		BorderForeground(lipgloss.Color("69"))

	paneTitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("223")).
		Bold(true)

	mutedTextStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	var b strings.Builder

	titleText := "Select a repository"
	if m.openMode || m.localOnly {
		titleText = "Select a repository to open"
	} else {
		titleText = "Select a repository to clone"
	}
	b.WriteString(headerStyle.Render(titleText))

	b.WriteString("\n\n")

	searchWidth := m.width - 2
	if searchWidth < 24 {
		searchWidth = 24
	}
	b.WriteString(searchBoxStyle.Width(searchWidth).Render(m.textinput.View()))
	b.WriteString("\n\n")

	if m.openMode {
		leftWidth := m.width / 2
		if leftWidth < 40 {
			leftWidth = m.width
		}
		rightWidth := m.width - leftWidth
		if rightWidth < 34 {
			rightWidth = 0
			leftWidth = m.width
		}

		leftContent := mutedTextStyle.Render("No repos found")
		if len(m.repoList.Items()) > 0 {
			leftContent = m.repoList.View()
		}

		leftPane := paneStyle
		if !m.focusWorktreePane {
			leftPane = focusedPaneStyle
		}
		leftPaneContent := paneTitleStyle.Render("Repositories ["+strings.ToUpper(m.currentFilterLabel())+"]") + "\n\n" + leftContent
		leftRendered := leftPane.Width(leftWidth - 1).Render(leftPaneContent)

		if rightWidth > 0 {
			rightPane := paneStyle
			if m.focusWorktreePane {
				rightPane = focusedPaneStyle
			}

			rightNormal := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
			rightSelected := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
			rightCreate := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
			rightOpenBadge := lipgloss.NewStyle().
				Foreground(lipgloss.Color("81")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)
			rightMuted := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

			var right strings.Builder
			repo := m.selectedRepoFromList()
			right.WriteString(paneTitleStyle.Render("Worktrees"))
			right.WriteString("\n\n")
			if repo == nil {
				right.WriteString(rightMuted.Render("Select a repository to view worktrees"))
			} else {
				options := m.worktreeOptionsForRepo(repo)
				if len(options) == 1 && options[0] == "+ Create new worktree" {
					right.WriteString(rightMuted.Render("No worktrees yet"))
					right.WriteString("\n\n")
				}
				idx := m.worktreeSelection[repo.FullName]
				if idx < 0 || idx >= len(options) {
					idx = 0
				}
				for i, option := range options {
					isSelected := m.focusWorktreePane && i == idx
					isCreate := option == "+ Create new worktree"
					label := option
					if !isCreate {
						if openedByRepo, ok := m.openedWorktrees[repo.FullName]; ok {
							if openedByRepo[option] {
								label = option + " " + rightOpenBadge.Render("[open]")
							}
						}
					}

					line := "  " + label
					if isSelected {
						right.WriteString(rightSelected.Render(line))
					} else if isCreate {
						right.WriteString(rightCreate.Render(line))
					} else {
						right.WriteString(rightNormal.Render(line))
					}
					right.WriteString("\n\n")
				}
				if m.creatingWorktree {
					right.WriteString("\n")
					right.WriteString(rightMuted.Render("Create inline: name[:base]"))
					right.WriteString("\n")
					right.WriteString(m.worktreeInput.View())
					if strings.TrimSpace(m.worktreeInputHint) != "" {
						right.WriteString("\n")
						right.WriteString(rightMuted.Render(m.worktreeInputHint))
					}
				}
			}

			rightRendered := rightPane.Width(rightWidth - 1).Render(right.String())
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftRendered, rightRendered))
		} else {
			b.WriteString(leftRendered)
		}
	} else {
		body := mutedTextStyle.Render("No repos found")
		if len(m.repoList.Items()) > 0 {
			body = m.repoList.View()
		}
		panelWidth := m.width - 1
		if panelWidth < 40 {
			panelWidth = 40
		}
		b.WriteString(paneStyle.Width(panelWidth).Render(paneTitleStyle.Render("Repositories ["+strings.ToUpper(m.currentFilterLabel())+"]") + "\n\n" + body))
	}

	keybinds := []string{"scope: " + strings.ToUpper(m.currentFilterLabel()), "↑/↓ move"}
	if m.allowLocalToggle {
		keybinds = append([]string{"tab: scope"}, keybinds...)
	}
	if m.openMode {
		keybinds = append(keybinds, "←/→ pane")
	}
	if m.creatingWorktree {
		keybinds = append(keybinds, "name[:base]", "enter create")
	} else if m.openMode && m.focusWorktreePane {
		keybinds = append(keybinds, "enter open/create")
	} else if m.openMode || m.localOnly {
		keybinds = append(keybinds, "enter open")
	} else {
		keybinds = append(keybinds, "enter clone")
	}
	keybinds = append(keybinds, "esc cancel")

	keybindBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Foreground(lipgloss.Color("245")).
		Padding(0, 1)

	b.WriteString("\n\n")
	keybindWidth := m.width - 1
	if keybindWidth < 40 {
		keybindWidth = 40
	}
	b.WriteString(keybindBoxStyle.Width(keybindWidth).Render(strings.Join(keybinds, "  |  ")))

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
		switch m.filterIndex {
		case 1:
			if !m.localRepos[repo.FullName] {
				continue
			}
		case 2:
			if !m.openedRepos[repo.FullName] {
				continue
			}
		}
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
			IsOpen:  m.openedRepos[repo.FullName],
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
	width          int
	height         int
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
		width:         80,
		height:        20,
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if msg.Width > 4 {
			m.featureInput.Width = msg.Width - 4
		}
		return m, nil

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
		start, end := featurePromptVisibleRange(len(m.baseBranches), m.baseIndex, m.baseListHeight())
		if start > 0 {
			b.WriteString(instructionStyle.Render(fmt.Sprintf("  ... %d above", start)))
			b.WriteString("\n")
		}
		for i := start; i < end; i++ {
			branch := m.baseBranches[i]
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
		if end < len(m.baseBranches) {
			b.WriteString(instructionStyle.Render(fmt.Sprintf("  ... %d below", len(m.baseBranches)-end)))
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

func (m featurePromptModel) baseListHeight() int {
	if len(m.baseBranches) == 0 {
		return 1
	}
	listHeight := m.height - 10
	if listHeight < 4 {
		listHeight = 4
	}
	if listHeight > len(m.baseBranches) {
		listHeight = len(m.baseBranches)
	}
	return listHeight
}

func featurePromptVisibleRange(total int, selected int, visible int) (start int, end int) {
	if total <= 0 {
		return 0, 0
	}
	if visible <= 0 || visible >= total {
		return 0, total
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	start = selected - (visible / 2)
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > total {
		end = total
		start = end - visible
	}
	if start < 0 {
		start = 0
	}
	return start, end
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

type CloneWorktreeSpec struct {
	Name       string
	BaseBranch string
}

type CloneWorktreePlan struct {
	CreateDefault bool
	CreateReview  bool
	Custom        []CloneWorktreeSpec
}

type cloneWorktreeOptionsModel struct {
	defaultBranch  string
	branches       []string
	cursor         int
	createDefault  bool
	createReview   bool
	customWorktree []CloneWorktreeSpec
	requestAdd     bool
	cancelled      bool
	quitting       bool
}

func newCloneWorktreeOptionsModel(defaultBranch string, branches []string, defaultChecked bool) cloneWorktreeOptionsModel {
	defaultBranch = strings.TrimSpace(defaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	if len(branches) == 0 {
		branches = []string{defaultBranch}
	}
	return cloneWorktreeOptionsModel{
		defaultBranch:  defaultBranch,
		branches:       branches,
		createDefault:  defaultChecked,
		createReview:   defaultChecked,
		customWorktree: make([]CloneWorktreeSpec, 0),
		cursor:         0,
	}
}

func (m cloneWorktreeOptionsModel) optionCount() int {
	return 2 + len(m.customWorktree) + 1
}

func (m cloneWorktreeOptionsModel) customStartIndex() int {
	return 2
}

func (m cloneWorktreeOptionsModel) addRowIndex() int {
	return m.optionCount() - 1
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
			m.cursor = (m.cursor - 1 + m.optionCount()) % m.optionCount()
		case tea.KeyDown, tea.KeyCtrlN:
			m.cursor = (m.cursor + 1) % m.optionCount()
		case tea.KeySpace:
			switch m.cursor {
			case 0:
				m.createDefault = !m.createDefault
			case 1:
				m.createReview = !m.createReview
			}
		case tea.KeyEnter:
			if m.cursor == m.addRowIndex() {
				m.requestAdd = true
				m.quitting = true
				return m, tea.Quit
			}

			if m.cursor >= m.customStartIndex() && m.cursor < m.addRowIndex() {
				idx := m.cursor - m.customStartIndex()
				if idx >= 0 && idx < len(m.customWorktree) {
					m.customWorktree = append(m.customWorktree[:idx], m.customWorktree[idx+1:]...)
					if m.cursor >= m.optionCount() {
						m.cursor = m.optionCount() - 1
					}
				}
				return m, nil
			}

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

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Bold(true)
	instructionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("120")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	var b strings.Builder
	b.WriteString(headerStyle.Render("Clone worktree options"))
	b.WriteString("\n\n")

	renderCheckbox := func(index int, label string, checked bool) {
		prefix := "  "
		if index == m.cursor {
			prefix = cursorStyle.Render("▶ ")
		}
		box := "[ ]"
		if checked {
			box = selectedStyle.Render("[✓]")
		}
		b.WriteString(prefix)
		b.WriteString(box)
		b.WriteString(" ")
		b.WriteString(normalStyle.Render(label))
		b.WriteString("\n")
	}

	renderCheckbox(0, fmt.Sprintf("Default branch worktree (%s)", m.defaultBranch), m.createDefault)
	renderCheckbox(1, fmt.Sprintf("Review worktree (from %s)", m.defaultBranch), m.createReview)

	for i, custom := range m.customWorktree {
		index := m.customStartIndex() + i
		prefix := "  "
		if index == m.cursor {
			prefix = cursorStyle.Render("▶ ")
		}
		label := fmt.Sprintf("Custom worktree: %s (from %s)", custom.Name, custom.BaseBranch)
		b.WriteString(prefix)
		b.WriteString(selectedStyle.Render("[✓]"))
		b.WriteString(" ")
		b.WriteString(normalStyle.Render(label))
		b.WriteString("\n")
	}

	addPrefix := "  "
	if m.cursor == m.addRowIndex() {
		addPrefix = cursorStyle.Render("▶ ")
	}
	b.WriteString(addPrefix)
	b.WriteString(normalStyle.Render("+ Add custom worktree"))
	b.WriteString("\n\n")
	b.WriteString(instructionStyle.Render("up/down: navigate | space: toggle default/review | enter: add/remove/confirm | esc: cancel"))

	return b.String()
}

func RunCloneWorktreePlanPrompt(branches []string, defaultBranch string, defaultChecked bool) (CloneWorktreePlan, bool, error) {
	m := newCloneWorktreeOptionsModel(defaultBranch, branches, defaultChecked)

	for {
		p := tea.NewProgram(m, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return CloneWorktreePlan{}, false, fmt.Errorf("failed to run clone worktree options prompt: %w", err)
		}

		final, ok := finalModel.(cloneWorktreeOptionsModel)
		if !ok {
			return CloneWorktreePlan{}, false, fmt.Errorf("unexpected model type")
		}

		if final.cancelled {
			return CloneWorktreePlan{}, true, nil
		}

		if !final.requestAdd {
			return CloneWorktreePlan{
				CreateDefault: final.createDefault,
				CreateReview:  final.createReview,
				Custom:        append([]CloneWorktreeSpec(nil), final.customWorktree...),
			}, false, nil
		}

		featureBranch, baseBranch, cancelled, err := RunCreateWorktreePrompt(final.branches, final.defaultBranch)
		if err != nil {
			return CloneWorktreePlan{}, false, err
		}

		final.requestAdd = false
		final.quitting = false

		if !cancelled {
			featureBranch = strings.TrimSpace(featureBranch)
			baseBranch = strings.TrimSpace(baseBranch)
			if baseBranch == "" {
				baseBranch = final.defaultBranch
			}
			if featureBranch != "" && !containsCloneWorktreeSpec(final.customWorktree, featureBranch) {
				final.customWorktree = append(final.customWorktree, CloneWorktreeSpec{Name: featureBranch, BaseBranch: baseBranch})
			}
		}

		m = final
	}
}

func containsCloneWorktreeSpec(list []CloneWorktreeSpec, name string) bool {
	for _, item := range list {
		if item.Name == name {
			return true
		}
	}
	return false
}

func RunCloneWorktreeOptionsPrompt(defaultBranch string) (createDefault bool, createReview bool, addCustom bool, cancelled bool, err error) {
	plan, cancelled, err := RunCloneWorktreePlanPrompt([]string{defaultBranch}, defaultBranch, true)
	if err != nil {
		return true, true, false, false, err
	}
	if cancelled {
		return false, false, false, true, nil
	}
	return plan.CreateDefault, plan.CreateReview, len(plan.Custom) > 0, false, nil
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
	return RunOpenFuzzySearchWithOpened(repos, localRepos, nil, nil, nil)
}

func RunOpenFuzzySearchWithOpened(repos []github.Repo, localRepos map[string]bool, openedRepos map[string]bool, openedWorktrees map[string]map[string]bool, worktreesByRepo map[string][]string) (*FuzzySearchResult, error) {
	m := newModelWithControls(repos, false, localRepos, true, true, false)
	if openedRepos != nil {
		m.openedRepos = openedRepos
	}
	if openedWorktrees != nil {
		m.openedWorktrees = openedWorktrees
	}
	if worktreesByRepo != nil {
		m.repoWorktrees = worktreesByRepo
	}
	m.repoList.SetItems(m.filterRepos(m.textinput.Value()))

	p := tea.NewProgram(
		m,
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
			SelectedWorktree: m.selectedWorktree,
			CreateWorktree:   m.createWorktree,
			WorktreeBase:     m.createWorktreeBase,
		}, nil
	}

	return nil, fmt.Errorf("no repository selected")
}
