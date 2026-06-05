package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
)

// Phase 1 pins the planned runtime dependencies while later phases add real TUI and watcher code.
var dependencyAnchors = []any{
	tea.Quit,
	key.NewBinding,
	lipgloss.NewStyle,
	fsnotify.Create,
}
