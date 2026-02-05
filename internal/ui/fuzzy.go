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
	repos         []github.Repo
	repoList      list.Model
	textinput     textinput.Model
	quitting      bool
	selected      *github.Repo
	worktree      bool
	localOnly     bool
	localRepos    map[string]bool
	openMode      bool
	currentPage   page
	settingsIndex int
	worktrees     []string
	worktreeIndex int
	width         int
	height        int
	lastInput     string
}

func newModel(repos []github.Repo, worktree bool, localRepos map[string]bool, openMode bool) model {
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
		repos:         repos,
		textinput:     ti,
		repoList:      l,
		worktree:      worktree,
		localRepos:    localRepos,
		openMode:      openMode,
		localOnly:     false,
		currentPage:   pageMain,
		settingsIndex: 0,
		width:         80,
		height:        20,
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
		if m.currentPage == pageMain {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
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
				m.localOnly = !m.localOnly
				m.repoList.SetItems(m.filterRepos(m.textinput.Value()))
				m.repoList.ResetSelected()
				return m, nil

			case tea.KeyDown, tea.KeyCtrlN:
				m.repoList.CursorDown()

			case tea.KeyUp, tea.KeyCtrlP:
				m.repoList.CursorUp()

			case tea.KeyCtrlS:
				m.currentPage = pageSettings
				return m, nil
			}
		} else if m.currentPage == pageWorktrees {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
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
			case tea.KeyCtrlC, tea.KeyEsc:
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

	if m.localOnly {
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
	instructions = append(instructions, "tab: toggle local")
	instructions = append(instructions, "ctrl+s: settings")
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
		{"Sesh open mode", m.openMode},
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

	return nil, fmt.Errorf("no repository selected")
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
