package chat

import (
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestSystemPromptPriorityLogic(t *testing.T) {
	// Create initial config
	config := &configuration.Config{
		ChatModel:           "llama3.2",
		OllamaURL:           "http://localhost:11434",
		DefaultSystemPrompt: "Original default prompt",
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     make(map[string]int),
	}

	ctx := t.Context()
	model := NewModel(ctx, config)

	// Test 1: Initial state should use default prompt and not be marked as manual
	if model.sessionSystemPrompt != "Original default prompt" {
		t.Errorf("Expected session prompt to be initialized with default, got %q", model.sessionSystemPrompt)
	}
	if model.sessionSystemPromptManual {
		t.Error("Expected session prompt manual flag to be false initially")
	}

	// Test 2: When default is updated and session wasn't manually modified, it should update
	newConfig := &configuration.Config{
		ChatModel:           "llama3.2",
		OllamaURL:           "http://localhost:11434",
		DefaultSystemPrompt: "Updated default prompt",
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     make(map[string]int),
	}

	model.UpdateFromConfiguration(ctx, newConfig)

	if model.sessionSystemPrompt != "Updated default prompt" {
		t.Errorf("Expected session prompt to be updated to new default, got %q", model.sessionSystemPrompt)
	}
	if model.sessionSystemPromptManual {
		t.Error("Expected session prompt manual flag to still be false after automatic update")
	}

	// Test 3: Manually modify the session prompt
	model.sessionSystemPrompt = "Manually edited prompt"
	model.sessionSystemPromptManual = true

	// Test 4: When default is updated again, manual session prompt should take precedence
	newerConfig := &configuration.Config{
		ChatModel:           "llama3.2",
		OllamaURL:           "http://localhost:11434",
		DefaultSystemPrompt: "Another updated default prompt",
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     make(map[string]int),
	}

	model.UpdateFromConfiguration(ctx, newerConfig)

	if model.sessionSystemPrompt != "Manually edited prompt" {
		t.Errorf("Expected session prompt to remain manually edited, got %q", model.sessionSystemPrompt)
	}
	if !model.sessionSystemPromptManual {
		t.Error("Expected session prompt manual flag to remain true")
	}

	// Test 5: If user manually saves the same prompt as default, manual flag should reset
	model.sessionSystemPrompt = "Another updated default prompt"
	model.sessionSystemPromptManual = false // This would be set in the save logic when prompt matches default

	// Test that it can be updated again since manual flag is reset
	finalConfig := &configuration.Config{
		ChatModel:           "llama3.2",
		OllamaURL:           "http://localhost:11434",
		DefaultSystemPrompt: "Final default prompt",
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     make(map[string]int),
	}

	model.UpdateFromConfiguration(ctx, finalConfig)

	if model.sessionSystemPrompt != "Final default prompt" {
		t.Errorf("Expected session prompt to be updated after manual flag reset, got %q", model.sessionSystemPrompt)
	}
}
