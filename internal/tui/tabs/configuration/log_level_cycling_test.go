package configuration

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevensen/gollama-chat/internal/configuration"
)

// TestLogLevelCycling tests that the LogLevel field cycles through valid values
func TestLogLevelCycling(t *testing.T) {
	// Create initial config with a specific log level
	initialConfig := configuration.DefaultConfig()
	initialConfig.LogLevel = "debug"

	// Create configuration tab model
	model := NewModel(initialConfig)

	// Set active field to LogLevelField
	model.activeField = LogLevelField

	// Test cycling through all log levels
	expectedLevels := []string{"info", "warn", "error", "debug"} // debug -> info -> warn -> error -> debug

	for i, expectedLevel := range expectedLevels {
		// Simulate pressing Enter to cycle to next level
		keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
		updatedModel, _ := model.Update(keyMsg)
		model = updatedModel.(Model)

		// Check that the log level has changed to the expected value
		if model.editConfig.LogLevel != expectedLevel {
			t.Errorf("Cycle %d: expected log level '%s', got '%s'", i+1, expectedLevel, model.editConfig.LogLevel)
		}
	}
}

// TestLogLevelCyclingFromDifferentStartingPoints tests cycling from each valid log level
func TestLogLevelCyclingFromDifferentStartingPoints(t *testing.T) {
	testCases := []struct {
		startLevel string
		expected   string
	}{
		{"debug", "info"},
		{"info", "warn"},
		{"warn", "error"},
		{"error", "debug"},
	}

	for _, tc := range testCases {
		t.Run("from_"+tc.startLevel, func(t *testing.T) {
			// Create config with specific starting log level
			config := configuration.DefaultConfig()
			config.LogLevel = tc.startLevel

			// Create model and set active field
			model := NewModel(config)
			model.activeField = LogLevelField

			// Cycle once
			keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
			updatedModel, _ := model.Update(keyMsg)
			model = updatedModel.(Model)

			// Check result
			if model.editConfig.LogLevel != tc.expected {
				t.Errorf("Starting from '%s': expected '%s', got '%s'",
					tc.startLevel, tc.expected, model.editConfig.LogLevel)
			}
		})
	}
}

// TestLogLevelCyclingWithSpace tests that Space key also cycles log levels
func TestLogLevelCyclingWithSpace(t *testing.T) {
	// Create initial config
	initialConfig := configuration.DefaultConfig()
	initialConfig.LogLevel = "warn"

	// Create model and set active field
	model := NewModel(initialConfig)
	model.activeField = LogLevelField

	// Simulate pressing Space to cycle
	keyMsg := tea.KeyMsg{Type: tea.KeySpace}
	updatedModel, _ := model.Update(keyMsg)
	model = updatedModel.(Model)

	// Should cycle from "warn" to "error"
	if model.editConfig.LogLevel != "error" {
		t.Errorf("Expected log level 'error', got '%s'", model.editConfig.LogLevel)
	}
}

// TestLogLevelNotEditableWithTextInput tests that LogLevel field doesn't enter text editing mode
func TestLogLevelNotEditableWithTextInput(t *testing.T) {
	// Create initial config
	initialConfig := configuration.DefaultConfig()
	initialConfig.LogLevel = "info"

	// Create model and set active field to LogLevel
	model := NewModel(initialConfig)
	model.activeField = LogLevelField

	// Try to start editing with Enter (this should cycle, not enter edit mode)
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(keyMsg)
	model = updatedModel.(Model)

	// Should not be in editing mode
	if model.editing {
		t.Error("LogLevelField should not enter text editing mode")
	}

	// Should have cycled from "info" to "warn"
	if model.editConfig.LogLevel != "warn" {
		t.Errorf("Expected log level 'warn', got '%s'", model.editConfig.LogLevel)
	}
}
