package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestASCIIArtDisplayLogic(t *testing.T) {
	// Create a test configuration
	config := &configuration.Config{
		ChatModel:           "llama3.1",
		DefaultSystemPrompt: "You are a helpful assistant",
		SelectedCollections: make(map[string]bool),
	}

	ctx := context.Background()

	tests := []struct {
		name           string
		width          int
		height         int
		expectASCII    bool
		expectFallback bool
	}{
		{
			name:           "large screen should show ASCII art",
			width:          80,
			height:         40,
			expectASCII:    true,
			expectFallback: false,
		},
		{
			name:           "short screen should show fallback message",
			width:          80,
			height:         25,
			expectASCII:    false,
			expectFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh model for each test
			model := NewModel(ctx, config)

			// Set the model dimensions using the Update method
			updatedModel, _ := model.Update(tea.WindowSizeMsg{
				Width:  tt.width,
				Height: tt.height,
			})
			model = updatedModel.(Model)

			// Render the view (no messages should be present on startup)
			rendered := model.View()

			// Check if ASCII art is present (check for a distinctive line instead of full art)
			asciiLine := ":+********************************#*=." // Distinctive line from ASCII art
			containsASCII := strings.Contains(rendered, asciiLine)

			// Check if fallback message is present
			fallbackMessage := "No messages yet. Type a message and press Enter to start chatting!"
			containsFallback := strings.Contains(rendered, fallbackMessage)

			if tt.expectASCII && !containsASCII {
				t.Errorf("Expected ASCII art to be displayed for dimensions %dx%d, but it wasn't found", tt.width, tt.height)
			}

			if !tt.expectASCII && containsASCII {
				t.Errorf("Did not expect ASCII art to be displayed for dimensions %dx%d, but it was found", tt.width, tt.height)
			}

			if tt.expectFallback && !containsFallback {
				t.Errorf("Expected fallback message to be displayed for dimensions %dx%d, but it wasn't found", tt.width, tt.height)
			}

			if !tt.expectFallback && containsFallback {
				t.Errorf("Did not expect fallback message to be displayed for dimensions %dx%d, but it was found", tt.width, tt.height)
			}
		})
	}
}

func TestASCIIArtNotDisplayedWhenMessagesExist(t *testing.T) {
	// Create a test configuration
	config := &configuration.Config{
		ChatModel:           "llama3.1",
		DefaultSystemPrompt: "You are a helpful assistant",
		SelectedCollections: make(map[string]bool),
	}

	ctx := context.Background()
	model := NewModel(ctx, config)

	// Set large screen dimensions using Update method
	updatedModel, _ := model.Update(tea.WindowSizeMsg{
		Width:  80,
		Height: 40,
	})
	model = updatedModel.(Model)

	// Add a message (not startup state)
	model.messages = []Message{
		{
			Role:    "user",
			Content: "Hello, world!",
			Time:    time.Now(),
		},
	}

	// Render the messages view using the View method
	rendered := model.View()

	// Check that ASCII art is NOT present
	asciiLine := ":+********************************#*=." // Distinctive line from ASCII art
	if strings.Contains(rendered, asciiLine) {
		t.Error("ASCII art should not be displayed when messages exist")
	}

	// Check that the user message is present
	if !strings.Contains(rendered, "Hello, world!") {
		t.Error("User message should be displayed when messages exist")
	}
}
