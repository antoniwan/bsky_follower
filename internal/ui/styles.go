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
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(highlight).
		PaddingLeft(2).
		PaddingRight(2).
		MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
		Foreground(special).
		PaddingLeft(2).
		PaddingRight(2).
		MarginBottom(1)

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