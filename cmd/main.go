package main

import (
	"context"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/tui"
)

func main() {
	ctx := context.Background()
	// Load configuration
	config, err := configuration.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create TUI model
	model := tui.NewModel(ctx, config)

	// Create Bubble Tea program with performance optimizations
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithFPS(60),    // Higher FPS for more responsive input
		tea.WithInputTTY(), // Use TTY input for better responsiveness
	)

	// Run the program
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
