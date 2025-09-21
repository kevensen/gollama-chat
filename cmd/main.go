package main

import (
	"context"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/core"
)

func main() {
	ctx := context.Background()

	// Load configuration
	config, err := configuration.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create TUI model
	model := core.NewModel(ctx, config)

	// Create Bubble Tea program with balanced performance optimizations
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithFPS(60),            // Balanced FPS - responsive but not excessive
		tea.WithInputTTY(),         // Use TTY input for better responsiveness
		tea.WithoutSignalHandler(), // Disable signal handling for less overhead
	)

	// Run the program
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
