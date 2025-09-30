package core

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/configuration/utils/connection"
	ragTab "github.com/kevensen/gollama-chat/internal/tui/tabs/rag"
)

func TestNewModel(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()

	model := NewModel(ctx, config)

	// Test basic initialization
	if model.config != config {
		t.Error("Config should be set correctly")
	}

	if model.activeTab != ChatTab {
		t.Error("Active tab should default to ChatTab")
	}

	expectedTabs := []string{"Chat", "Settings", "RAG Collections", "Tools"}
	if len(model.tabs) != len(expectedTabs) {
		t.Errorf("Expected %d tabs, got %d", len(expectedTabs), len(model.tabs))
	}

	for i, expected := range expectedTabs {
		if model.tabs[i] != expected {
			t.Errorf("Tab %d: expected %q, got %q", i, expected, model.tabs[i])
		}
	}

	// Test that child models are initialized
	// Note: We can't easily test the internal state of child models without exposing them
	// but we can verify the constructor doesn't panic and returns a valid model
}

func TestTabEnum(t *testing.T) {
	tests := []struct {
		name     string
		tab      Tab
		expected int
	}{
		{"ChatTab", ChatTab, 0},
		{"ConfigTab", ConfigTab, 1},
		{"RAGTab", RAGTab, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.tab) != tt.expected {
				t.Errorf("Expected %s to be %d, got %d", tt.name, tt.expected, int(tt.tab))
			}
		})
	}
}

func TestModel_Init(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	cmd := model.Init()

	// Test that Init returns a command (should be a batch command)
	if cmd == nil {
		t.Error("Init() should return a command")
	}

	// We can't easily test the specific commands without complex mocking,
	// but we can verify it doesn't panic
}

func TestModel_Update_WindowSize(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Test window size message
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, cmd := model.Update(msg)

	newModel := updatedModel.(Model)

	// Test that dimensions are set to 90% of terminal size
	expectedWidth := int(float64(100) * 0.90) // 90
	expectedHeight := int(float64(50) * 0.90) // 45

	if newModel.width != expectedWidth {
		t.Errorf("Expected width %d, got %d", expectedWidth, newModel.width)
	}

	if newModel.height != expectedHeight {
		t.Errorf("Expected height %d, got %d", expectedHeight, newModel.height)
	}

	// Note: Commands might be nil if child models don't return commands
	// This is normal behavior for window resize messages
	_ = cmd
}

func TestModel_Update_TabSwitching(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	tests := []struct {
		name          string
		keyMsg        tea.KeyMsg
		startTab      Tab
		expectedTab   Tab
		shouldHaveCmd bool
	}{
		{
			name:          "tab forward from chat",
			keyMsg:        tea.KeyMsg{Type: tea.KeyTab},
			startTab:      ChatTab,
			expectedTab:   ConfigTab,
			shouldHaveCmd: false,
		},
		{
			name:          "tab forward from config",
			keyMsg:        tea.KeyMsg{Type: tea.KeyTab},
			startTab:      ConfigTab,
			expectedTab:   RAGTab,
			shouldHaveCmd: true, // RAG tab initialization
		},
		{
			name:          "tab forward from RAG (to Tools)",
			keyMsg:        tea.KeyMsg{Type: tea.KeyTab},
			startTab:      RAGTab,
			expectedTab:   ToolsTab,
			shouldHaveCmd: false,
		},
		{
			name:          "tab forward from Tools (wrap around)",
			keyMsg:        tea.KeyMsg{Type: tea.KeyTab},
			startTab:      ToolsTab,
			expectedTab:   ChatTab,
			shouldHaveCmd: false,
		},
		{
			name:          "shift+tab backward from chat (wrap around)",
			keyMsg:        tea.KeyMsg{Type: tea.KeyShiftTab},
			startTab:      ChatTab,
			expectedTab:   ToolsTab,
			shouldHaveCmd: false,
		},
		{
			name:          "shift+tab backward from config",
			keyMsg:        tea.KeyMsg{Type: tea.KeyShiftTab},
			startTab:      ConfigTab,
			expectedTab:   ChatTab,
			shouldHaveCmd: false,
		},
		{
			name:          "shift+tab backward from RAG",
			keyMsg:        tea.KeyMsg{Type: tea.KeyShiftTab},
			startTab:      RAGTab,
			expectedTab:   ConfigTab,
			shouldHaveCmd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.activeTab = tt.startTab
			updatedModel, cmd := model.Update(tt.keyMsg)
			newModel := updatedModel.(Model)

			if newModel.activeTab != tt.expectedTab {
				t.Errorf("Expected active tab %d, got %d", tt.expectedTab, newModel.activeTab)
			}

			if tt.shouldHaveCmd && cmd == nil {
				t.Error("Expected command to be returned for RAG tab initialization")
			}
		})
	}
}

func TestModel_Update_QuitKeys(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	quitKeys := []tea.KeyMsg{
		{Type: tea.KeyCtrlC},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	}

	for _, keyMsg := range quitKeys {
		t.Run("quit_key_"+keyMsg.String(), func(t *testing.T) {
			_, cmd := model.Update(keyMsg)

			// Note: We can't directly test tea.Quit without more complex setup
			// The important thing is that it doesn't panic and handles the key
			_ = cmd
		})
	}
}

func TestModel_Update_InputHandling(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	var cmd tea.Cmd
	// Test space character input
	spaceMsg := tea.KeyMsg{Type: tea.KeySpace}
	model.Update(spaceMsg)

	// Input will now go through normal message routing
	// We're testing that it doesn't panic

	// Test ASCII character input
	charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	_, cmd = model.Update(charMsg)

	// Input goes through normal chat model update
	// We're mainly testing that it doesn't panic
	_ = cmd
}

func TestModel_Update_ConfigurationMessages(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Test ConfigUpdatedMsg
	newConfig := &configuration.Config{
		ChatModel:      "updated-model",
		EmbeddingModel: "updated-embedding",
		OllamaURL:      "http://localhost:11435",
	}
	configMsg := ragTab.ConfigUpdatedMsg{Config: newConfig}

	updatedModel, cmd := model.Update(configMsg)
	newModel := updatedModel.(Model)

	if newModel.config != newConfig {
		t.Error("Config should be updated when ConfigUpdatedMsg is received")
	}

	if cmd == nil {
		t.Error("ConfigUpdatedMsg should return a command")
	}
}

func TestModel_Update_CollectionsMessages(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Test CollectionsUpdatedMsg
	collectionsMsg := ragTab.CollectionsUpdatedMsg{
		SelectedCollections: []string{"collection1", "collection2"},
	}

	_, cmd := model.Update(collectionsMsg)

	// Note: Command might be nil depending on internal state
	// The important thing is that it doesn't panic
	_ = cmd
}

func TestModel_Update_ConnectionMessages(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Test connection check message
	connectionMsg := connection.CheckMsg{}

	_, cmd := model.Update(connectionMsg)

	// Note: Command might be nil depending on internal state
	// The important thing is that it doesn't panic
	_ = cmd
}

func TestModel_View_MinimalTerminal(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 20

	tests := []struct {
		name           string
		height         int
		shouldContain  string
		shouldNotPanic bool
	}{
		{
			name:           "height 1",
			height:         1,
			shouldContain:  "Chat", // Should contain tab content
			shouldNotPanic: true,
		},
		{
			name:           "height 2",
			height:         2,
			shouldContain:  "...", // Should contain minimal indicator
			shouldNotPanic: true,
		},
		{
			name:           "height 3",
			height:         3,
			shouldContain:  "Chat", // Should contain tab bar
			shouldNotPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.height = tt.height

			defer func() {
				if r := recover(); r != nil && tt.shouldNotPanic {
					t.Errorf("View() panicked with height %d: %v", tt.height, r)
				}
			}()

			view := model.View()

			if tt.shouldContain != "" && view == "" {
				t.Errorf("View should not be empty for height %d", tt.height)
			}

			// Basic sanity check - view should not be excessively long
			if len(view) > 1000 {
				t.Errorf("View seems excessively long (%d chars) for minimal terminal", len(view))
			}
		})
	}
}

func TestModel_View_NormalSize(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80
	model.height = 24

	// Test view for each tab
	tabs := []Tab{ChatTab, ConfigTab, RAGTab}
	tabNames := []string{"Chat", "Config", "RAG"}

	for i, tab := range tabs {
		t.Run("tab_"+tabNames[i], func(t *testing.T) {
			model.activeTab = tab
			view := model.View()

			if view == "" {
				t.Errorf("View should not be empty for %s tab", tabNames[i])
			}

			// View should contain tab bar
			if len(view) < 10 {
				t.Errorf("View seems too short for %s tab", tabNames[i])
			}
		})
	}
}

func TestModel_RenderTabBar(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	tests := []struct {
		name          string
		width         int
		activeTab     Tab
		shouldContain []string
	}{
		{
			name:          "normal width",
			width:         80,
			activeTab:     ChatTab,
			shouldContain: []string{"Chat", "Settings", "RAG Collections", "Tools"},
		},
		{
			name:          "narrow width",
			width:         25,
			activeTab:     ConfigTab,
			shouldContain: []string{"Chat", "Config", "RAG", "Tools"}, // Medium names
		},
		{
			name:          "very narrow width",
			width:         10,
			activeTab:     RAGTab,
			shouldContain: []string{"C", "S", "R", "T"}, // Single letter tabs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.width = tt.width
			model.activeTab = tt.activeTab

			tabBar := model.renderTabBar()

			if tabBar == "" {
				t.Error("Tab bar should not be empty")
			}

			// Check for expected content
			for _, expected := range tt.shouldContain {
				if !containsAny(tabBar, expected) {
					t.Errorf("Tab bar should contain %q", expected)
				}
			}
		})
	}
}

func TestModel_RenderFooter(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	tests := []struct {
		name          string
		width         int
		activeTab     Tab
		shouldContain string
	}{
		{
			name:          "narrow terminal",
			width:         40,
			activeTab:     ChatTab,
			shouldContain: "Tab: Switch",
		},
		{
			name:          "medium terminal",
			width:         60,
			activeTab:     ConfigTab,
			shouldContain: "Tab/Shift+Tab",
		},
		{
			name:          "wide terminal chat",
			width:         100,
			activeTab:     ChatTab,
			shouldContain: "Enter: Send",
		},
		{
			name:          "wide terminal config",
			width:         100,
			activeTab:     ConfigTab,
			shouldContain: "Enter: Edit",
		},
		{
			name:          "wide terminal RAG",
			width:         100,
			activeTab:     RAGTab,
			shouldContain: "Space: Toggle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.width = tt.width
			model.activeTab = tt.activeTab

			footer := model.renderFooter()

			if footer == "" {
				t.Error("Footer should not be empty")
			}

			if !containsAny(footer, tt.shouldContain) {
				t.Errorf("Footer should contain %q", tt.shouldContain)
			}
		})
	}
}

func TestModel_SyncRAGCollections(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Test that syncRAGCollections doesn't panic
	// We can't easily test the actual synchronization without complex mocking
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("syncRAGCollections() panicked: %v", r)
		}
	}()

	model.syncRAGCollections()
}

// Helper function to check if a string contains any of the expected substrings
func containsAny(s string, substrings ...string) bool {
	for _, substring := range substrings {
		if len(substring) > 0 {
			for i := 0; i <= len(s)-len(substring); i++ {
				if s[i:i+len(substring)] == substring {
					return true
				}
			}
		}
	}
	return false
}
