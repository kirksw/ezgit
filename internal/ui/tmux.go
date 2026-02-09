package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tmuxSessionItem struct {
	name string
}

func (i tmuxSessionItem) FilterValue() string { return i.name }

type tmuxSessionDelegate struct{}

func (d tmuxSessionDelegate) Height() int                             { return 1 }
func (d tmuxSessionDelegate) Spacing() int                            { return 0 }
func (d tmuxSessionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d tmuxSessionDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(tmuxSessionItem)
	if !ok {
		return
	}

	var style lipgloss.Style
	if index == m.Index() {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	} else {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	}
	fmt.Fprint(w, style.Render(item.name))
}

type tmuxSessionModel struct {
	sessions  []string
	list      list.Model
	input     textinput.Model
	selected  string
	cancelled bool
	quitting  bool
	lastInput string
}

func newTmuxSessionModel(sessions []string) tmuxSessionModel {
	input := textinput.New()
	input.Placeholder = "Search sessions..."
	input.Focus()
	input.CharLimit = 256
	input.Width = 80

	l := list.New(nil, tmuxSessionDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.SetShowPagination(false)
	l.SetWidth(80)
	l.SetHeight(12)

	m := tmuxSessionModel{
		sessions: sessions,
		list:     l,
		input:    input,
	}
	m.list.SetItems(m.filterSessions(""))
	return m
}

func (m tmuxSessionModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tmuxSessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		listHeight := msg.Height - 8
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
		case tea.KeyEnter:
			if len(m.list.Items()) > 0 {
				if item := m.list.SelectedItem(); item != nil {
					if si, ok := item.(tmuxSessionItem); ok {
						m.selected = si.name
					}
				}
			}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyDown, tea.KeyCtrlN:
			m.list.CursorDown()
			return m, nil
		case tea.KeyUp, tea.KeyCtrlP:
			m.list.CursorUp()
			return m, nil
		}
	}

	input, cmd := m.input.Update(msg)
	m.input = input

	if current := m.input.Value(); current != m.lastInput {
		m.lastInput = current
		m.list.SetItems(m.filterSessions(current))
		m.list.ResetSelected()
	}

	return m, cmd
}

func (m tmuxSessionModel) View() string {
	if m.quitting {
		return ""
	}

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Bold(true)
	instructionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)

	var b strings.Builder
	b.WriteString(headerStyle.Render("Select tmux session"))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	if len(m.list.Items()) > 0 {
		b.WriteString(m.list.View())
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("No sessions found"))
	}

	b.WriteString("\n\n")
	b.WriteString(instructionStyle.Render("up/down: navigate | enter: connect | esc: cancel"))
	return b.String()
}

func (m tmuxSessionModel) filterSessions(query string) []list.Item {
	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]list.Item, 0, len(m.sessions))
	for _, session := range m.sessions {
		if query != "" && !strings.Contains(strings.ToLower(session), query) {
			continue
		}
		items = append(items, tmuxSessionItem{name: session})
	}
	return items
}

func RunTmuxSessionSearch(sessions []string) (selected string, cancelled bool, err error) {
	p := tea.NewProgram(
		newTmuxSessionModel(sessions),
		tea.WithAltScreen(),
	)
	finalModel, err := p.Run()
	if err != nil {
		return "", false, fmt.Errorf("failed to run tmux session search: %w", err)
	}

	m, ok := finalModel.(tmuxSessionModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected model type")
	}
	if m.cancelled {
		return "", true, nil
	}
	return strings.TrimSpace(m.selected), false, nil
}
