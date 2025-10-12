package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
	"github.com/kevensen/gollama-chat/internal/tui/core"
)

func main() {
	// Parse command line flags
	flag.Parse()
	ctx := context.Background()

	// Load configuration first to get logging settings
	config, err := configuration.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logging based on configuration
	logConfig := &logging.Config{
		Level:        logging.LogLevel(config.GetLogLevel()),
		EnableFile:   config.EnableFileLogging,
		LogDir:       logging.DefaultDir(),
		EnableStderr: false, // Disable stderr logging for TUI mode
	}

	if err := logging.Initialize(logConfig); err != nil {
		log.Printf("Failed to initialize logging: %v", err)
		// Continue with default logging
	} else {
		// Set up cleanup for graceful shutdown
		defer logging.Close()
	}

	// Log application startup
	logger := logging.WithComponent("main")
	logger.Info("Starting gollama-chat application")

	// Run TUI mode
	logger.Debug("Running in TUI mode")
	runTUIMode(ctx, config)
}

func runTUIMode(ctx context.Context, config *configuration.Config) {
	logger := logging.WithComponent("tui")
	logger.Info("Initializing TUI mode")

	// Create TUI model
	model := core.NewModel(ctx, config)

	// Create Bubble Tea program
	logger.Debug("Configuring TUI")
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithFPS(60),            // Balanced FPS - responsive but not excessive
		tea.WithInputTTY(),         // Use TTY input for better responsiveness
		tea.WithoutSignalHandler(), // Disable signal handling for less overhead
	)

	logger.Info("Starting TUI program")
	// Run the program
	if _, err := program.Run(); err != nil {
		logger.Error("Error running TUI program", "error", err)
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	logger.Info("TUI program ended")
}
