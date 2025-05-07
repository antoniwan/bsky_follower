package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	ready bool
	width int
	height int
}

func NewModel() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Title
	title := titleStyle.Render("🦋 Bluesky Follower")
	b.WriteString(title + "\n")

	// Subtitle
	subtitle := subtitleStyle.Render("Automated follower management for Bluesky")
	b.WriteString(subtitle + "\n")

	// Info Box
	info := fmt.Sprintf("Press 'q' to quit\nWindow size: %d x %d", m.width, m.height)
	box := boxStyle.Render(info)
	b.WriteString(box)

	return b.String()
} 