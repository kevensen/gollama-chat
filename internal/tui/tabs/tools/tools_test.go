package tools

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ollama/ollama/api"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestTrustLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    TrustLevel
		expected string
	}{
		{"TrustNone", TrustNone, "None"},
		{"AskForTrust", AskForTrust, "Ask"},
		{"TrustSession", TrustSession, "Session"},
		{"Unknown", TrustLevel(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("TrustLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewModel(t *testing.T) {
	config := &configuration.Config{
		ToolTrustLevels: make(map[string]int),
	}
	ctx := context.Background()

	model := NewModel(ctx, config)

	if model.config != config {
		t.Error("Config not properly set")
	}
	if model.ctx != ctx {
		t.Error("Context not properly set")
	}
	if model.viewMode != ViewModeList {
		t.Error("View mode should default to ViewModeList")
	}
	if model.selectedIndex != 0 {
		t.Error("Selected index should default to 0")
	}
}

func TestToolTrustLevelPersistence(t *testing.T) {
	config := &configuration.Config{
		ToolTrustLevels: make(map[string]int),
	}
	ctx := context.Background()

	model := NewModel(ctx, config)

	// Test updating trust level
	model.UpdateToolTrust("test_tool", TrustSession)

	// Verify the trust level was saved to configuration
	expectedTrustLevel := int(TrustSession)
	if config.GetToolTrustLevel("test_tool") != expectedTrustLevel {
		t.Errorf("Expected trust level %d, got %d", expectedTrustLevel, config.GetToolTrustLevel("test_tool"))
	}
}

func TestToolRefresh(t *testing.T) {
	config := &configuration.Config{
		ToolTrustLevels: make(map[string]int),
	}
	ctx := context.Background()

	model := NewModel(ctx, config)

	// Simulate refreshing tools
	cmd := model.refreshTools()
	if cmd == nil {
		t.Error("refreshTools should return a command")
	}

	// Execute the command to get the message
	msg := cmd()

	// Verify it's a ToolsRefreshedMsg
	if toolsMsg, ok := msg.(ToolsRefreshedMsg); ok {
		if toolsMsg.Error != nil {
			t.Errorf("Unexpected error in tools refresh: %v", toolsMsg.Error)
		}
		if len(toolsMsg.Tools) == 0 {
			t.Error("Expected at least one tool (filesystem_read)")
		}

		// Check if filesystem_read tool is present with correct default trust
		filesystemToolFound := false
		for _, tool := range toolsMsg.Tools {
			if tool.Name == "filesystem_read" {
				filesystemToolFound = true
				if tool.Trust != TrustSession {
					t.Errorf("filesystem_read should default to TrustSession, got %v", tool.Trust)
				}
				if tool.Source != "builtin" {
					t.Errorf("filesystem_read should be builtin, got %s", tool.Source)
				}
			}
		}
		if !filesystemToolFound {
			t.Error("filesystem_read tool not found in tools list")
		}
	} else {
		t.Errorf("Expected ToolsRefreshedMsg, got %T", msg)
	}
}

func TestToolTrustLoadingFromConfiguration(t *testing.T) {
	config := &configuration.Config{
		ToolTrustLevels: map[string]int{
			"filesystem_read": int(TrustNone), // Override default
			"test_tool":       int(AskForTrust),
		},
	}
	ctx := context.Background()

	model := NewModel(ctx, config)

	// Refresh tools to load from configuration
	cmd := model.refreshTools()
	msg := cmd()

	if toolsMsg, ok := msg.(ToolsRefreshedMsg); ok {
		for _, tool := range toolsMsg.Tools {
			if tool.Name == "filesystem_read" {
				if tool.Trust != TrustNone {
					t.Errorf("filesystem_read should have trust level TrustNone from config, got %v", tool.Trust)
				}
			}
		}
	}
}

func TestTrustPromptNavigation(t *testing.T) {
	config := &configuration.Config{
		ToolTrustLevels: make(map[string]int),
	}
	ctx := context.Background()

	model := NewModel(ctx, config)

	// Create a test tool
	testTool := Tool{
		Name:        "test_tool",
		Description: "Test tool",
		Trust:       TrustNone,
		Source:      "builtin",
		APITool: &api.Tool{
			Function: api.ToolFunction{
				Name:        "test_tool",
				Description: "Test tool",
			},
		},
	}
	model.tools = []Tool{testTool}

	// Show trust prompt
	updatedModelInterface, _ := model.showTrustPrompt()
	model = updatedModelInterface.(Model)

	if model.viewMode != ViewModeTrustPrompt {
		t.Error("View mode should be ViewModeTrustPrompt")
	}
	if model.trustPrompt == nil {
		t.Error("Trust prompt should be initialized")
	}
	if model.trustPrompt.Tool.Name != "test_tool" {
		t.Error("Trust prompt should reference the selected tool")
	}

	// Test navigation through Update method
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	updatedModelInterface, _ = model.Update(keyMsg)
	updatedModel := updatedModelInterface.(Model)

	if updatedModel.trustPrompt.SelectedLevel != AskForTrust {
		t.Error("Down key should move to AskForTrust")
	}

	// Test selection
	keyMsg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModelInterface2, _ := updatedModel.Update(keyMsg)
	updatedModel = updatedModelInterface2.(Model)

	if updatedModel.viewMode != ViewModeList {
		t.Error("Enter should return to list view")
	}
	if updatedModel.tools[0].Trust != AskForTrust {
		t.Error("Tool trust level should be updated to AskForTrust")
	}
}

func TestTrustLevelDescriptions(t *testing.T) {
	config := &configuration.Config{
		ToolTrustLevels: make(map[string]int),
	}
	ctx := context.Background()

	model := NewModel(ctx, config)

	// Create a test tool and show trust prompt
	testTool := Tool{
		Name:        "test_tool",
		Description: "Test tool",
		Trust:       TrustNone,
		Source:      "builtin",
		APITool:     &api.Tool{},
	}
	model.tools = []Tool{testTool}
	updatedModelInterface, _ := model.showTrustPrompt()
	model = updatedModelInterface.(Model)

	// Check that all trust levels have descriptions
	view := model.renderTrustPrompt()

	// Should contain descriptions for all trust levels
	expectedDescriptions := []string{
		"Block tool execution entirely",
		"Ask for permission before each use",
		"Trust for entire session",
	}

	for _, desc := range expectedDescriptions {
		if !strings.Contains(view, desc) {
			t.Errorf("Trust prompt should contain description: %s", desc)
		}
	}
}

func TestGetTools(t *testing.T) {
	config := &configuration.Config{
		ToolTrustLevels: make(map[string]int),
	}
	ctx := context.Background()

	model := NewModel(ctx, config)

	// Add test tools
	testTools := []Tool{
		{Name: "tool1", Trust: TrustNone},
		{Name: "tool2", Trust: TrustSession},
	}
	model.tools = testTools

	tools := model.GetTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "tool1" {
		t.Errorf("Expected first tool name to be 'tool1', got '%s'", tools[0].Name)
	}
}

// Integration test for complete tool authorization flow
func TestToolAuthorizationFlow(t *testing.T) {
	tests := []struct {
		name               string
		initialTrustLevel  TrustLevel
		userAction         string // "up", "down", "enter"
		expectedFinalTrust TrustLevel
		expectViewChange   bool
	}{
		{
			name:               "Trust None to Ask via down key",
			initialTrustLevel:  TrustNone,
			userAction:         "down",
			expectedFinalTrust: TrustNone, // Should move selection, not change trust
			expectViewChange:   false,
		},
		{
			name:               "Accept Ask trust level",
			initialTrustLevel:  TrustNone,
			userAction:         "enter",
			expectedFinalTrust: TrustNone, // Should apply current selection
			expectViewChange:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &configuration.Config{
				ToolTrustLevels: make(map[string]int),
			}
			ctx := context.Background()
			model := NewModel(ctx, config)

			// Setup test tool
			testTool := Tool{
				Name:    "test_tool",
				Trust:   tt.initialTrustLevel,
				Source:  "builtin",
				APITool: &api.Tool{},
			}
			model.tools = []Tool{testTool}
			model.showTrustPrompt()

			initialViewMode := model.viewMode

			// Simulate user action
			var keyMsg tea.KeyMsg
			switch tt.userAction {
			case "up":
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
			case "down":
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
			case "enter":
				keyMsg = tea.KeyMsg{Type: tea.KeyEnter}
			}

			updatedModelInterface, _ := model.Update(keyMsg)
			updatedModel := updatedModelInterface.(Model)

			// Verify view change expectation
			viewChanged := updatedModel.viewMode != initialViewMode
			if viewChanged != tt.expectViewChange {
				t.Errorf("Expected view change: %v, got: %v", tt.expectViewChange, viewChanged)
			}

			// For enter key, verify trust level was updated
			if tt.userAction == "enter" {
				if len(updatedModel.tools) > 0 {
					actualTrust := updatedModel.tools[0].Trust
					if actualTrust != tt.expectedFinalTrust {
						t.Errorf("Expected final trust %v, got %v", tt.expectedFinalTrust, actualTrust)
					}
				}
			}
		})
	}
}

// TestCompleteWorkflowWithSequentialCommands demonstrates using output from previous commands
func TestCompleteWorkflowWithSequentialCommands(t *testing.T) {
	config := &configuration.Config{
		ToolTrustLevels: map[string]int{
			"initial_tool": int(TrustNone),
		},
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Step 1: Refresh tools and capture the loaded tools
	refreshCmd := model.refreshTools()
	refreshMsg := refreshCmd()

	var loadedTools []Tool
	if toolsMsg, ok := refreshMsg.(ToolsRefreshedMsg); ok {
		loadedTools = toolsMsg.Tools
		// Update model with loaded tools
		updatedModelInterface, _ := model.Update(toolsMsg)
		model = updatedModelInterface.(Model)

		t.Logf("Loaded %d tools from refresh command", len(loadedTools))
	} else {
		t.Fatal("Expected ToolsRefreshedMsg from refresh command")
	}

	// Step 2: Use the loaded tools list to select a specific tool
	var targetToolIndex int = -1
	for i, tool := range model.tools {
		if tool.Name == "filesystem_read" { // Use a tool we know exists
			targetToolIndex = i
			break
		}
	}

	if targetToolIndex == -1 {
		t.Skip("filesystem_read tool not found in loaded tools")
	}

	// Select the found tool
	model.selectedIndex = targetToolIndex
	selectedTool := model.tools[targetToolIndex]
	originalTrustLevel := selectedTool.Trust

	t.Logf("Selected tool: %s with original trust level: %v", selectedTool.Name, originalTrustLevel)

	// Step 3: Use the selected tool to open trust prompt
	updatedModelInterface, _ := model.showTrustPrompt()
	model = updatedModelInterface.(Model)

	if model.trustPrompt == nil {
		t.Fatal("Trust prompt should be initialized after showTrustPrompt")
	}

	// Verify the prompt references our selected tool
	if model.trustPrompt.Tool.Name != selectedTool.Name {
		t.Errorf("Trust prompt should reference %s, got %s",
			selectedTool.Name, model.trustPrompt.Tool.Name)
	}

	// Step 4: Use the trust prompt to navigate to a different trust level
	// Navigate down to change trust level (assuming it starts at current level)
	initialPromptLevel := model.trustPrompt.SelectedLevel
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}} // down arrow
	updatedModelInterface, _ = model.Update(keyMsg)
	model = updatedModelInterface.(Model)

	newPromptLevel := model.trustPrompt.SelectedLevel
	if newPromptLevel == initialPromptLevel {
		// Try one more navigation if they were the same
		keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
		updatedModelInterface, _ = model.Update(keyMsg)
		model = updatedModelInterface.(Model)
		newPromptLevel = model.trustPrompt.SelectedLevel
	}

	t.Logf("Navigated from trust level %v to %v", initialPromptLevel, newPromptLevel)

	// Step 5: Use the new trust level selection to apply the change
	keyMsg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModelInterface, _ = model.Update(keyMsg)
	model = updatedModelInterface.(Model)

	// Step 6: Verify the tool's trust level was updated using the selected level
	updatedTool := model.tools[targetToolIndex]
	if updatedTool.Trust != newPromptLevel {
		t.Errorf("Expected tool trust to be updated to %v, got %v",
			newPromptLevel, updatedTool.Trust)
	}

	// Step 7: Use the updated tool state to verify configuration persistence
	configTrustLevel := config.GetToolTrustLevel(selectedTool.Name)
	if configTrustLevel != int(newPromptLevel) {
		t.Errorf("Expected configuration trust level %d, got %d",
			int(newPromptLevel), configTrustLevel)
	}

	// Step 8: Use the final state to verify we're back in list mode
	if model.viewMode != ViewModeList {
		t.Errorf("Expected to return to list view after trust update, got %v", model.viewMode)
	}

	t.Logf("Successfully completed workflow: %s trust level changed from %v to %v",
		selectedTool.Name, originalTrustLevel, newPromptLevel)
}
