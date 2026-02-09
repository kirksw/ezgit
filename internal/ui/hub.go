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

type HubMode int

const (
	HubModeClone HubMode = iota
	HubModeOpen
	HubModeConnect
)

type HubResult struct {
	Mode      HubMode
	Repo      *github.Repo
	Session   string
	Worktree  bool
	Convert   bool
	Cancelled bool
}

type hubAction int

const (
	hubActionNone hubAction = iota
	hubActionEnter
	hubActionConvert
)

type hubSessionItem struct {
	Name string
}

func (i hubSessionItem) FilterValue() string {
	return i.Name
}

type hubItemDelegate struct{}

func (d hubItemDelegate) Height() int                             { return 2 }
func (d hubItemDelegate) Spacing() int                            { return 1 }
func (d hubItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d hubItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	var style lipgloss.Style
	if index == m.Index() {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	} else {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	}

	switch item := listItem.(type) {
	case repoItem:
		prefix := ""
		if item.IsLocal {
			prefix = "* "
		}
		text := fmt.Sprintf("%s%s/%s", prefix, item.Owner, item.Name)
		if item.Description != "" {
			text += fmt.Sprintf("\n  %s", truncateString(item.Description, 60))
		}
		fmt.Fprint(w, style.Render(text))
	case hubSessionItem:
		fmt.Fprint(w, style.Render(item.Name))
	default:
		fmt.Fprint(w, style.Render("?"))
	}
}

type hubModel struct {
	allRepos     []github.Repo
	openRepos    []github.Repo
	localRepos   map[string]bool
	sessions     []string
	mode         HubMode
	worktree     bool
	input        textinput.Model
	list         list.Model
	width        int
	height       int
	lastInput    string
	selectedRepo *github.Repo
	selectedTmux string
	action       hubAction
	cancelled    bool
	quitting     bool
}

func newHubModel(
	allRepos []github.Repo,
	openRepos []github.Repo,
	localRepos map[string]bool,
	sessions []string,
	defaultWorktree bool,
) hubModel {
	input := textinput.New()
	input.Placeholder = "Search..."
	input.Focus()
	input.CharLimit = 256
	input.Width = 80

	l := list.New(nil, hubItemDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.SetShowPagination(false)
	l.SetWidth(80)
	l.SetHeight(12)

	m := hubModel{
		allRepos:   allRepos,
		openRepos:  openRepos,
		localRepos: localRepos,
		sessions:   sessions,
		mode:       HubModeClone,
		worktree:   defaultWorktree,
		input:      input,
		list:       l,
		width:      80,
		height:     20,
		action:     hubActionNone,
	}
	m.refreshItems()
	return m
}

func (m hubModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m hubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width)
		listHeight := msg.Height - 9
		if listHeight < 4 {
			listHeight = 4
		}
		m.list.SetHeight(listHeight)
		m.input.Width = msg.Width - 4
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit
		case tea.KeyLeft, tea.KeyShiftTab:
			m.mode = (m.mode + 2) % 3
			m.refreshItems()
			return m, nil
		case tea.KeyRight, tea.KeyTab:
			m.mode = (m.mode + 1) % 3
			m.refreshItems()
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			m.list.CursorDown()
			return m, nil
		case tea.KeyUp, tea.KeyCtrlP:
			m.list.CursorUp()
			return m, nil
		case tea.KeyEnter:
			if !m.captureSelection(hubActionEnter) {
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		}

		key := strings.ToLower(strings.TrimSpace(msg.String()))
		switch key {
		case "1":
			m.mode = HubModeClone
			m.refreshItems()
			return m, nil
		case "2":
			m.mode = HubModeOpen
			m.refreshItems()
			return m, nil
		case "3":
			m.mode = HubModeConnect
			m.refreshItems()
			return m, nil
		case "w":
			if m.mode == HubModeClone {
				m.worktree = !m.worktree
				return m, nil
			}
		case "c":
			if m.mode == HubModeOpen {
				if !m.captureSelection(hubActionConvert) {
					return m, nil
				}
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	input, cmd := m.input.Update(msg)
	m.input = input
	if current := m.input.Value(); current != m.lastInput {
		m.lastInput = current
		m.refreshItems()
	}

	return m, cmd
}

func (m *hubModel) captureSelection(action hubAction) bool {
	item := m.list.SelectedItem()
	if item == nil {
		return false
	}

	switch m.mode {
	case HubModeClone, HubModeOpen:
		ri, ok := item.(repoItem)
		if !ok {
			return false
		}
		repo := ri.Repo
		m.selectedRepo = &repo
		m.action = action
		return true
	case HubModeConnect:
		si, ok := item.(hubSessionItem)
		if !ok {
			return false
		}
		m.selectedTmux = si.Name
		m.action = action
		return true
	default:
		return false
	}
}

func (m *hubModel) refreshItems() {
	m.list.SetItems(m.filterItems(m.input.Value()))
	m.list.ResetSelected()
}

func (m hubModel) filterItems(query string) []list.Item {
	query = strings.ToLower(strings.TrimSpace(query))
	switch m.mode {
	case HubModeClone:
		items := make([]list.Item, 0, len(m.allRepos))
		for _, repo := range m.allRepos {
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
	case HubModeOpen:
		items := make([]list.Item, 0, len(m.openRepos))
		for _, repo := range m.openRepos {
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
				IsLocal: true,
			})
		}
		return items
	case HubModeConnect:
		items := make([]list.Item, 0, len(m.sessions))
		for _, session := range m.sessions {
			if query != "" && !strings.Contains(strings.ToLower(session), query) {
				continue
			}
			items = append(items, hubSessionItem{Name: session})
		}
		return items
	default:
		return nil
	}
}

func (m hubModel) View() string {
	if m.quitting {
		return ""
	}

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("120")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	instructionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)

	var modeLabel strings.Builder
	modes := []struct {
		mode HubMode
		text string
	}{
		{HubModeClone, "1 Clone"},
		{HubModeOpen, "2 Open"},
		{HubModeConnect, "3 Connect"},
	}
	for i, entry := range modes {
		if i > 0 {
			modeLabel.WriteString("  |  ")
		}
		if entry.mode == m.mode {
			modeLabel.WriteString(selectedStyle.Render("▶ " + entry.text))
		} else {
			modeLabel.WriteString(normalStyle.Render("  " + entry.text))
		}
	}

	title := ""
	switch m.mode {
	case HubModeClone:
		title = "Clone repositories"
	case HubModeOpen:
		title = "Open local repositories"
	case HubModeConnect:
		title = "Connect tmux sessions"
	}

	instructions := ""
	switch m.mode {
	case HubModeClone:
		worktreeState := "off"
		if m.worktree {
			worktreeState = "on"
		}
		instructions = fmt.Sprintf("up/down: navigate | tab/left/right: mode | w: worktree (%s) | enter: clone | esc: cancel", worktreeState)
	case HubModeOpen:
		instructions = "up/down: navigate | tab/left/right: mode | c: convert | enter: open | esc: cancel"
	case HubModeConnect:
		instructions = "up/down: navigate | tab/left/right: mode | enter: connect | esc: cancel"
	}

	emptyText := "No items found"
	switch m.mode {
	case HubModeClone:
		emptyText = "No repositories found"
	case HubModeOpen:
		emptyText = "No local repositories found"
	case HubModeConnect:
		emptyText = "No tmux sessions found"
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render("ezgit tui"))
	b.WriteString("\n")
	b.WriteString(modeLabel.String())
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	if len(m.list.Items()) > 0 {
		b.WriteString(m.list.View())
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(emptyText))
	}

	b.WriteString("\n\n")
	b.WriteString(instructionStyle.Render(instructions))
	return b.String()
}

func RunHub(
	allRepos []github.Repo,
	openRepos []github.Repo,
	localRepos map[string]bool,
	sessions []string,
	defaultWorktree bool,
) (*HubResult, error) {
	p := tea.NewProgram(
		newHubModel(allRepos, openRepos, localRepos, sessions, defaultWorktree),
		tea.WithAltScreen(),
	)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run hub: %w", err)
	}

	m, ok := finalModel.(hubModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	result := &HubResult{
		Mode:      m.mode,
		Worktree:  m.worktree,
		Cancelled: m.cancelled,
	}
	if m.cancelled {
		return result, nil
	}
	if m.action == hubActionConvert {
		result.Convert = true
	}
	if m.selectedRepo != nil {
		result.Repo = m.selectedRepo
	}
	if m.selectedTmux != "" {
		result.Session = m.selectedTmux
	}
	return result, nil
}
