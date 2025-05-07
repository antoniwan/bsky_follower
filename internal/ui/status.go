package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// StatusMsg represents a status message
type StatusMsg struct {
	Message string
	Type    StatusType
	Time    time.Time
}

// StatusType represents the type of status message
type StatusType int

const (
	StatusInfo StatusType = iota
	StatusSuccess
	StatusError
)

// StatusCmd represents a command to show a status message
func StatusCmd(message string, statusType StatusType) tea.Cmd {
	return func() tea.Msg {
		return StatusMsg{
			Message: message,
			Type:    statusType,
			Time:    time.Now(),
		}
	}
}

// GetStatusStyle returns the style for a status message
func GetStatusStyle(statusType StatusType) string {
	switch statusType {
	case StatusSuccess:
		return "✓"
	case StatusError:
		return "✗"
	default:
		return "ℹ"
	}
}

// FormatStatus formats a status message
func FormatStatus(msg StatusMsg) string {
	style := GetStatusStyle(msg.Type)
	return fmt.Sprintf("%s %s", style, msg.Message)
} 