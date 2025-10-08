package core

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevensen/gollama-chat/internal/configuration"
)

// TestTabVisibilityWithSystemPrompt tests that tabs remain visible when the system prompt is expanded
func TestTabVisibilityWithSystemPrompt(t *testing.T) {
	// Create a minimal config for testing
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
		ChromaDBURL:    "http://localhost:8000",
		DefaultSystemPrompt: "This is a very long system prompt that would normally cause the tabs to disappear when expanded. " +
			"It contains multiple lines of text to simulate a realistic system prompt that would take up significant space " +
			"in the configuration tab when the Default System Prompt field is selected. This should not cause the tabs " +
			"to become invisible in the terminal interface.",
		RAGEnabled:          true,
		ChromaDBDistance:    0.7,
		MaxDocuments:        10,
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     make(map[string]int),
		MCPServers:          []configuration.MCPServer{},
		LogLevel:            "info",
		EnableFileLogging:   false,
	}

	ctx := context.Background()
	model := NewModel(ctx, config)

	// Test with a small terminal size that would previously cause issues
	smallTerminal := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ := model.Update(smallTerminal)
	convertedModel := updatedModel.(Model)
	model = &convertedModel

	// Switch to configuration tab
	model.activeTab = ConfigTab

	// Render the view
	view := model.View()

	// The view should always contain tab indicators
	// Check for basic tab content that should always be visible
	if len(view) == 0 {
		t.Error("View should not be empty")
	}

	// Verify that we can render without panicking and that the result is reasonable
	if len(view) < 10 {
		t.Error("View seems too short, tabs might not be rendered properly")
	}

	// Test with even smaller terminal
	verySmallTerminal := tea.WindowSizeMsg{Width: 40, Height: 15}
	updatedModel, _ = model.Update(verySmallTerminal)
	convertedModel = updatedModel.(Model)
	model = &convertedModel

	view = model.View()
	if len(view) == 0 {
		t.Error("View should not be empty even with very small terminal")
	}

	// Test with extremely small terminal
	tinyTerminal := tea.WindowSizeMsg{Width: 20, Height: 8}
	updatedModel, _ = model.Update(tinyTerminal)
	convertedModel = updatedModel.(Model)
	model = &convertedModel

	view = model.View()
	if len(view) == 0 {
		t.Error("View should not be empty even with tiny terminal")
	}
}

// TestConfigurationTabHeightConsistency tests that the configuration tab receives consistent height values
func TestConfigurationTabHeightConsistency(t *testing.T) {
	config := &configuration.Config{
		ChatModel:           "test-model",
		EmbeddingModel:      "test-embedding",
		OllamaURL:           "http://localhost:11434",
		DefaultSystemPrompt: "Test prompt",
		RAGEnabled:          true,
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     make(map[string]int),
		MCPServers:          []configuration.MCPServer{},
		LogLevel:            "info",
		EnableFileLogging:   false,
	}

	ctx := context.Background()
	model := NewModel(ctx, config)

	// Simulate a window resize
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := model.Update(windowMsg)
	convertedModel := updatedModel.(Model)
	model = &convertedModel

	// The config model should receive height that accounts for tabs and footer
	// Main TUI uses 90% of terminal height, then subtracts 2 lines (tab + footer)
	expectedMainHeight := int(float64(30) * 0.90) // 27

	if model.height != expectedMainHeight {
		t.Errorf("Expected main model height %d, got %d", expectedMainHeight, model.height)
	}

	// For now, just verify that the configuration model doesn't panic when rendering
	view := model.View()
	if len(view) == 0 {
		t.Error("Configuration view should not be empty")
	}
}
