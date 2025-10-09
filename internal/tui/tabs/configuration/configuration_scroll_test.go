package configuration

import (
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestSystemPromptPanelToggle(t *testing.T) {
	// Create a test configuration
	config := &configuration.Config{
		ChatModel:           "test-model",
		EmbeddingModel:      "test-embedding",
		DefaultSystemPrompt: "This is a test system prompt for testing panel functionality.",
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

	// Test initial state - system prompt panel should be closed
	if model.showSystemPromptPanel {
		t.Error("Expected system prompt panel to be closed initially")
	}

	if model.systemPromptEditMode {
		t.Error("Expected system prompt edit mode to be false initially")
	}

	// Test opening system prompt panel (starts in view mode)
	model.activeField = DefaultSystemPromptField
	model.showSystemPromptPanel = true
	model.systemPromptEditInput = model.editConfig.DefaultSystemPrompt
	model.systemPromptEditCursor = 0
	model.systemPromptEditMode = false

	if !model.showSystemPromptPanel {
		t.Error("Expected system prompt panel to be open")
	}

	if model.systemPromptEditMode {
		t.Error("Expected system prompt panel to start in view mode")
	}

	// Test entering edit mode
	model.systemPromptEditMode = true
	model.systemPromptEditCursor = len(model.systemPromptEditInput)

	if !model.systemPromptEditMode {
		t.Error("Expected system prompt panel to be in edit mode")
	}

	// Test closing system prompt panel
	model.showSystemPromptPanel = false
	model.systemPromptEditInput = ""
	model.systemPromptEditCursor = 0
	model.systemPromptEditMode = false

	if model.showSystemPromptPanel {
		t.Error("Expected system prompt panel to be closed")
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
	config.DefaultSystemPrompt = "System prompt that no longer expands in place"

	// Create a model
	model := NewModel(config)
	model.width = 80
	model.height = 25

	// Test that the view renders without panicking
	view := model.View()

	if view == "" {
		t.Error("Expected view to render content, got empty string")
	}

	// Test with system prompt active (no longer expands in place)
	model.activeField = DefaultSystemPromptField
	compactView := model.View()

	if compactView == "" {
		t.Error("Expected compact view to render content, got empty string")
	}

	// Test with system prompt panel open
	model.showSystemPromptPanel = true
	model.systemPromptEditInput = config.DefaultSystemPrompt
	panelView := model.View()

	if panelView == "" {
		t.Error("Expected panel view to render content, got empty string")
	}
}
