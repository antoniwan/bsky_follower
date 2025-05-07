package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

	// Styles
	uiTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF69B4"))

	uiSubtitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A9A9A9"))

	uiMenuItemStyle = lipgloss.NewStyle().
		PaddingLeft(2)

	uiSelectedMenuItemStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#FF69B4")).
		Bold(true)

	uiDisabledMenuItemStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#808080"))

	uiStatusStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#00FF00"))

	uiHelpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A9A9A9"))

	infoStyle = lipgloss.NewStyle().
		Foreground(subtle).
		PaddingLeft(2).
		PaddingRight(2)

	boxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(highlight).
		Padding(1).
		MarginTop(1).
		MarginBottom(1)
) 