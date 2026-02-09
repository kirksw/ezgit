package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTmuxSessionModelCtrlCCancels(t *testing.T) {
	m := newTmuxSessionModel([]string{"dev", "ops"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(tmuxSessionModel)

	if !got.cancelled {
		t.Fatal("expected model to be cancelled")
	}
	if !got.quitting {
		t.Fatal("expected model to quit on ctrl+c")
	}
}

func TestTmuxSessionModelEnterSelectsSession(t *testing.T) {
	m := newTmuxSessionModel([]string{"dev", "ops"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(tmuxSessionModel)

	if got.selected != "dev" {
		t.Fatalf("selected=%q, want %q", got.selected, "dev")
	}
	if !got.quitting {
		t.Fatal("expected model to quit on enter")
	}
}
