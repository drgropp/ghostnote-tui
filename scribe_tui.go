package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drgropp/ghostnote-tui/scribe"

	tea "github.com/charmbracelet/bubbletea"
)

type scribeResultMsg struct {
	operation scribe.Operation
	path      string
	err       error
}

func (m model) runScribe(args []string) (model, tea.Cmd) {
	if len(args) != 1 {
		return m.flash("usage: :scribe summarize|tasks|stats", true)
	}
	operation, err := scribe.ParseOperation(args[0])
	if err != nil || !operation.SupportsText() {
		return m.flash("current note supports: summarize, tasks, stats", true)
	}
	if strings.TrimSpace(m.area.Value()) == "" {
		return m.flash("write or open a note before using scribe", true)
	}

	name := m.curName
	if name == "" {
		name = "untitled"
	}
	content := m.area.Value()
	m, clearStatus := m.flash("scribe "+string(operation)+" running", false)
	run := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		result, err := scribe.Default().RunText(ctx, operation, content)
		if err != nil {
			return scribeResultMsg{operation: operation, err: err}
		}
		path, err := writeScribeResult(name, operation, result.Markdown)
		return scribeResultMsg{operation: operation, path: path, err: err}
	}
	return m, tea.Batch(clearStatus, run)
}

func writeScribeResult(noteName string, operation scribe.Operation, markdown string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	directory := filepath.Join(home, ".ghostnote", "scribe")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create scribe result directory: %w", err)
	}
	filename := fmt.Sprintf(
		"%s-%s-%d.md",
		slug(noteName),
		operation,
		time.Now().UTC().UnixMilli(),
	)
	path := filepath.Join(directory, filename)
	if err := os.WriteFile(path, []byte(markdown), 0o644); err != nil {
		return "", fmt.Errorf("write scribe result: %w", err)
	}
	return path, nil
}
