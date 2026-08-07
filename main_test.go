package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBracketedPasteDoesNotTriggerCommandMode(t *testing.T) {
	m := initialModel()

	updated, _ := m.updateType(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(":copy this text"),
		Paste: true,
	})
	got := updated.(model)

	if got.mode != modeType {
		t.Fatalf("bracketed paste changed mode: got %v, want modeType", got.mode)
	}
	if got.area.Value() != ":copy this text" {
		t.Fatalf("pasted text = %q, want %q", got.area.Value(), ":copy this text")
	}
}

func TestCtrlVPasteIsForwardedToActiveInput(t *testing.T) {
	tests := []struct {
		name   string
		update func(model, tea.KeyMsg) (tea.Model, tea.Cmd)
	}{
		{
			name: "note editor",
			update: func(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
				return m.updateType(msg)
			},
		},
		{
			name: "command bar",
			update: func(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
				m.mode = modeCommand
				m.cmd.Focus()
				return m.updateCommand(msg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel()
			updated, cmd := tt.update(m, tea.KeyMsg{Type: tea.KeyCtrlV})
			got := updated.(model)

			if got.quitting {
				t.Fatal("Ctrl+V unexpectedly triggered quit")
			}
			if cmd == nil {
				t.Fatal("Ctrl+V was not forwarded to the active input's paste command")
			}
		})
	}
}

func TestCtrlCQuitBindingIsPreserved(t *testing.T) {
	m := initialModel()
	updated, cmd := m.updateType(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(model)

	if !got.quitting {
		t.Fatal("Ctrl+C no longer marks the application as quitting")
	}
	if cmd == nil {
		t.Fatal("Ctrl+C no longer returns the quit command")
	}
}
