package ui

import (
	"fmt"
	"strings"
	"time"

	"bsky_follower/internal/api"
	"bsky_follower/internal/logger"
	"bsky_follower/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	ready bool
	width int
	height int
	authenticated bool
	session *models.Session
	menuIndex int
	client *api.Client
	config *models.Config
	status *StatusMsg
}

func NewModel(config *models.Config) Model {
	return Model{
		menuIndex: 0,
		config: config,
		client: api.NewClient(config.Timeout, logger.GetAPILogger()),
	}
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

	case AuthMsg:
		if msg.Error != nil {
			m.status = &StatusMsg{
				Message: fmt.Sprintf("Authentication failed: %v", msg.Error),
				Type:    StatusError,
				Time:    time.Now(),
			}
			return m, nil
		}
		m.authenticated = true
		m.session = msg.Session
		m.status = &StatusMsg{
			Message: fmt.Sprintf("Successfully authenticated as %s", msg.Session.Handle),
			Type:    StatusSuccess,
			Time:    time.Now(),
		}
		return m, nil

	case StatusMsg:
		m.status = &msg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.menuIndex > 0 {
				m.menuIndex--
			}
			return m, nil
		case "down", "j":
			if m.menuIndex < 2 {
				m.menuIndex++
			}
			return m, nil
		case "enter":
			switch m.menuIndex {
			case 0: // Authenticate/Logout
				if m.authenticated {
					m.authenticated = false
					m.session = nil
					m.status = &StatusMsg{
						Message: "Successfully logged out",
						Type:    StatusSuccess,
						Time:    time.Now(),
					}
					return m, nil
				}
				return m, AuthCmd(m.client, m.config.Identifier, m.config.Password)
			case 1: // Fetch Users
				if !m.authenticated {
					m.status = &StatusMsg{
						Message: "Please authenticate first",
						Type:    StatusError,
						Time:    time.Now(),
					}
					return m, nil
				}
				// TODO: Implement fetch users
				return m, nil
			case 2: // Process Queue
				if !m.authenticated {
					m.status = &StatusMsg{
						Message: "Please authenticate first",
						Type:    StatusError,
						Time:    time.Now(),
					}
					return m, nil
				}
				// TODO: Implement process queue
				return m, nil
			}
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
	title := uiTitleStyle.Render("🦋 Bluesky Follower")
	b.WriteString(title + "\n")

	// Subtitle
	subtitle := uiSubtitleStyle.Render("Automated follower management for Bluesky")
	b.WriteString(subtitle + "\n\n")

	// Menu
	menuItems := []string{
		"Authenticate to BlueSky",
		"Fetch and Save Top Users",
		"Process Follow Queue",
	}

	if m.authenticated {
		menuItems[0] = fmt.Sprintf("Logout from BlueSky (%s)", m.session.Handle)
	}

	for i, item := range menuItems {
		style := uiMenuItemStyle
		if i == m.menuIndex {
			style = uiSelectedMenuItemStyle
		}
		if !m.authenticated && i > 0 {
			style = uiDisabledMenuItemStyle
		}
		b.WriteString(style.Render(item) + "\n")
	}

	// Status
	b.WriteString("\n")
	if m.status != nil {
		status := uiStatusStyle.Render(FormatStatus(*m.status))
		b.WriteString(status + "\n")
	} else if m.authenticated {
		status := uiStatusStyle.Render(fmt.Sprintf("Authenticated as: %s", m.session.Handle))
		b.WriteString(status + "\n")
	} else {
		status := uiStatusStyle.Render("Not authenticated")
		b.WriteString(status + "\n")
	}

	// Help
	help := uiHelpStyle.Render("↑/↓: Navigate • Enter: Select • q: Quit")
	b.WriteString("\n" + help)

	return b.String()
} 