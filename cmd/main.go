package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/core"
	"github.com/kevensen/gollama-chat/internal/webserver"
)

var (
	webPort = flag.Int("webport", 0, "Run in web mode on specified port (e.g., -webport 8080)")
	isChild = flag.Bool("child", false, "Internal flag - indicates running as child process")
)

func main() {
	// Parse command line flags
	flag.Parse()

	// If we're being run as a child process, force TUI mode
	if *isChild {
		// Create debug log file
		debugFile, err := os.OpenFile("/tmp/gollama-child-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			log.SetOutput(debugFile)
			defer debugFile.Close()
		}

		log.Printf("Child process starting...")

		ctx := context.Background()
		config, err := configuration.Load()
		if err != nil {
			log.Printf("Failed to load configuration: %v", err)
			log.Fatalf("Failed to load configuration: %v", err)
		}

		log.Printf("Configuration loaded successfully")

		runTUIMode(ctx, config)
		return
	}

	ctx := context.Background()

	// Load configuration
	config, err := configuration.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// If webport is specified and > 0, run in web mode
	if *webPort > 0 {
		runWebMode(ctx, config, *webPort)
	} else {
		runTUIMode(ctx, config)
	}
}

func runTUIMode(ctx context.Context, config *configuration.Config) {
	// Create TUI model
	model := core.NewModel(ctx, config)

	// Create Bubble Tea program with PTY-compatible settings
	var program *tea.Program
	if *isChild {
		// When running as child (in PTY), use minimal settings
		program = tea.NewProgram(
			model,
			tea.WithAltScreen(),
		)
	} else {
		// When running directly, use full settings
		program = tea.NewProgram(
			model,
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
			tea.WithFPS(60),            // Balanced FPS - responsive but not excessive
			tea.WithInputTTY(),         // Use TTY input for better responsiveness
			tea.WithoutSignalHandler(), // Disable signal handling for less overhead
		)
	}

	// Run the program
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func runWebMode(ctx context.Context, config *configuration.Config, port int) {
	server := webserver.New(port, config)
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}
}
