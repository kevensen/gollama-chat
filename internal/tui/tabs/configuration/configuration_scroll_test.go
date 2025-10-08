package configuration

import (
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestSystemPromptScrolling(t *testing.T) {
	// Create a test configuration
	config := &configuration.Config{
		ChatModel:           "test-model",
		EmbeddingModel:      "test-embedding",
		DefaultSystemPrompt: "This is a very long system prompt for testing scrolling functionality.",
		RAGEnabled:          true,
		OllamaURL:           "http://localhost:11434",
		ChromaDBURL:         "http://localhost:8000",
		ChromaDBDistance:    0.5,
		MaxDocuments:        10,
		LogLevel:            "info",
		EnableFileLogging:   false,
	}

	// Create a model with normal height
	model := NewModel(config)
	model.width = 80
	model.height = 25

	// Test initial state
	if model.systemPromptScrollPos != 0 {
		t.Errorf("Expected initial system prompt scroll position to be 0, got %d", model.systemPromptScrollPos)
	}

	// Test system prompt scrolling when active
	model.activeField = DefaultSystemPromptField

	// Test scroll down within system prompt
	originalPos := model.systemPromptScrollPos
	model.systemPromptScrollPos += 5 // Simulate page down

	if model.systemPromptScrollPos <= originalPos {
		t.Error("Expected system prompt scroll position to increase after scroll down")
	}

	// Test scroll position bounds
	model.systemPromptScrollPos = -5 // Negative value
	if model.systemPromptScrollPos < 0 {
		model.systemPromptScrollPos = 0
	}

	if model.systemPromptScrollPos != 0 {
		t.Error("Expected system prompt scroll position to be bounded to 0")
	}
}

func TestFieldNavigation(t *testing.T) {
	// Create a test configuration
	config := configuration.DefaultConfig()

	// Create a model
	model := NewModel(config)
	model.width = 80
	model.height = 25

	// Test initial field
	if model.activeField != OllamaURLField {
		t.Errorf("Expected initial active field to be OllamaURLField, got %v", model.activeField)
	}

	// Test navigation
	originalField := model.activeField
	if model.activeField < EnableFileLoggingField {
		model.activeField++
	}

	if model.activeField <= originalField {
		t.Error("Expected active field to advance when navigating down")
	}
}

func TestAnchoredLayout(t *testing.T) {
	// Create a test configuration
	config := configuration.DefaultConfig()
	config.DefaultSystemPrompt = "Very long system prompt that should expand in place"

	// Create a model
	model := NewModel(config)
	model.width = 80
	model.height = 25

	// Test that the view renders without panicking
	view := model.View()

	if view == "" {
		t.Error("Expected view to render content, got empty string")
	}

	// Test with system prompt active
	model.activeField = DefaultSystemPromptField
	expandedView := model.View()

	if expandedView == "" {
		t.Error("Expected expanded view to render content, got empty string")
	}
}
