package main

import (
	"fmt"
	"os"

	"bsky_follower/internal/config"
	"bsky_follower/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize UI
	model := ui.NewModel(cfg)
	program := tea.NewProgram(model)

	// Run the program
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
