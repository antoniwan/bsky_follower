package ui

import (
	"bsky_follower/internal/api"
	"bsky_follower/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

// AuthMsg represents an authentication message
type AuthMsg struct {
	Session *models.Session
	Error   error
}

// AuthCmd represents an authentication command
func AuthCmd(client *api.Client, identifier, password string) tea.Cmd {
	return func() tea.Msg {
		session, err := client.Login(identifier, password)
		return AuthMsg{
			Session: session,
			Error:   err,
		}
	}
} 