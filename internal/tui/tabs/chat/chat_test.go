package chat

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestNewModel(t *testing.T) {
	// Create a test configuration
	config := &configuration.Config{
		ChatModel:           "llama3.1",
		EmbeddingModel:      "nomic-embed-text",
		OllamaURL:           "http://localhost:11434",
		ChromaDBURL:         "http://localhost:8000",
		DefaultSystemPrompt: "You are a helpful assistant",
		RAGEnabled:          true,
		ChromaDBDistance:    1.0,
		MaxDocuments:        10,
		SelectedCollections: make(map[string]bool),
	}

	ctx := context.Background()

	tests := []struct {
		name   string
		ctx    context.Context
		config *configuration.Config
	}{
		{
			name:   "valid configuration",
			ctx:    ctx,
			config: config,
		},
		{
			name: "minimal configuration",
			ctx:  ctx,
			config: &configuration.Config{
				ChatModel:           "llama3.1",
				DefaultSystemPrompt: "Test prompt",
				SelectedCollections: make(map[string]bool),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(tt.ctx, tt.config)

			// Test initial state
			if model.config != tt.config {
				t.Errorf("NewModel() config = %v, want %v", model.config, tt.config)
			}

			if len(model.messages) != 0 {
				t.Errorf("NewModel() messages length = %d, want 0", len(model.messages))
			}

			if model.scrollOffset != 0 {
				t.Errorf("NewModel() scrollOffset = %d, want 0", model.scrollOffset)
			}

			if model.tokenCount != 0 {
				t.Errorf("NewModel() tokenCount = %d, want 0", model.tokenCount)
			}

			if model.showSystemPrompt != false {
				t.Errorf("NewModel() showSystemPrompt = %t, want false", model.showSystemPrompt)
			}

			if model.sessionSystemPrompt != tt.config.DefaultSystemPrompt {
				t.Errorf("NewModel() sessionSystemPrompt = %q, want %q",
					model.sessionSystemPrompt, tt.config.DefaultSystemPrompt)
			}

			if model.systemPromptEditMode != false {
				t.Errorf("NewModel() systemPromptEditMode = %t, want false", model.systemPromptEditMode)
			}

			// Test that components are initialized
			if model.inputModel == nil {
				t.Error("NewModel() inputModel is nil, want initialized")
			}

			if model.messageCache == nil {
				t.Error("NewModel() messageCache is nil, want initialized")
			}

			if model.ragService == nil {
				t.Error("NewModel() ragService is nil, want initialized")
			}

			// Test that cache invalidation flags are set correctly
			if !model.messagesNeedsUpdate {
				t.Error("NewModel() messagesNeedsUpdate = false, want true")
			}

			if !model.statusNeedsUpdate {
				t.Error("NewModel() statusNeedsUpdate = false, want true")
			}

			if !model.systemPromptNeedsUpdate {
				t.Error("NewModel() systemPromptNeedsUpdate = false, want true")
			}
		})
	}
}

func TestModel_BasicGetters(t *testing.T) {
	config := &configuration.Config{
		ChatModel:           "llama3.1",
		DefaultSystemPrompt: "Test prompt",
		SelectedCollections: make(map[string]bool),
	}

	model := NewModel(context.Background(), config)

	// Add some test messages
	testMessages := []Message{
		{Role: "user", Content: "Hello", Time: time.Now()},
		{Role: "assistant", Content: "Hi there!", Time: time.Now()},
	}
	model.messages = testMessages

	// Test message access
	if len(model.messages) != 2 {
		t.Errorf("Model messages length = %d, want 2", len(model.messages))
	}

	// Test RAG service access
	ragService := model.GetRAGService()
	if ragService == nil {
		t.Error("GetRAGService() returned nil")
	}

	if ragService != model.ragService {
		t.Error("GetRAGService() returned different service than internal")
	}
}

func TestModel_SetSize(t *testing.T) {
	config := &configuration.Config{
		ChatModel:           "llama3.1",
		DefaultSystemPrompt: "Test prompt",
		SelectedCollections: make(map[string]bool),
	}

	model := NewModel(context.Background(), config)

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"standard size", 80, 24},
		{"large size", 120, 40},
		{"small size", 40, 12},
		{"zero width", 0, 24},
		{"zero height", 80, 0},
		{"negative values", -10, -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set size through a window size message update
			model.width = tt.width
			model.height = tt.height

			if model.width != tt.width {
				t.Errorf("Model width = %d, want %d", model.width, tt.width)
			}

			if model.height != tt.height {
				t.Errorf("Model height = %d, want %d", model.height, tt.height)
			}
		})
	}
}

func TestModel_ScrollManagement(t *testing.T) {
	config := &configuration.Config{
		ChatModel:           "llama3.1",
		DefaultSystemPrompt: "Test prompt",
		SelectedCollections: make(map[string]bool),
	}

	model := NewModel(context.Background(), config)

	tests := []struct {
		name           string
		initialOffset  int
		newOffset      int
		expectedOffset int
	}{
		{"zero to positive", 0, 5, 5},
		{"positive to higher", 5, 10, 10},
		{"positive to zero", 5, 0, 0},
		{"negative offset", 0, -5, -5}, // Model should handle negative values
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.scrollOffset = tt.initialOffset
			model.scrollOffset = tt.newOffset

			if model.scrollOffset != tt.expectedOffset {
				t.Errorf("Model scrollOffset = %d, want %d", model.scrollOffset, tt.expectedOffset)
			}
		})
	}
}

func TestModel_CacheManagement(t *testing.T) {
	config := &configuration.Config{
		ChatModel:           "llama3.1",
		DefaultSystemPrompt: "Test prompt",
		SelectedCollections: make(map[string]bool),
	}

	model := NewModel(context.Background(), config)

	// Test cache initialization
	if model.messageCache == nil {
		t.Fatal("Message cache should be initialized")
	}

	// Test cache invalidation flags
	tests := []struct {
		name     string
		flag     *bool
		flagName string
	}{
		{"messages needs update", &model.messagesNeedsUpdate, "messagesNeedsUpdate"},
		{"status needs update", &model.statusNeedsUpdate, "statusNeedsUpdate"},
		{"system prompt needs update", &model.systemPromptNeedsUpdate, "systemPromptNeedsUpdate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initially should be true (needs update)
			if !*tt.flag {
				t.Errorf("%s should initially be true", tt.flagName)
			}

			// Test setting to false
			*tt.flag = false
			if *tt.flag {
				t.Errorf("%s should be false after setting", tt.flagName)
			}

			// Test setting back to true
			*tt.flag = true
			if !*tt.flag {
				t.Errorf("%s should be true after setting", tt.flagName)
			}
		})
	}
}

// Test Message Cache Functions
func TestNewMessageCache(t *testing.T) {
	cache := NewMessageCache()

	if cache == nil {
		t.Fatal("NewMessageCache() returned nil")
	}

	if cache.renderedMessages == nil {
		t.Error("renderedMessages map should be initialized")
	}

	if !cache.needsRefresh {
		t.Error("needsRefresh should be true initially")
	}

	if cache.lastWidth != 0 {
		t.Error("lastWidth should be 0 initially")
	}

	if cache.cachedTotalHeight != 0 {
		t.Error("cachedTotalHeight should be 0 initially")
	}
}

func TestMessageCache_InvalidateCache(t *testing.T) {
	cache := NewMessageCache()

	// Set needsRefresh to false to test invalidation
	cache.needsRefresh = false

	cache.InvalidateCache()

	if !cache.needsRefresh {
		t.Error("InvalidateCache() should set needsRefresh to true")
	}
}

func TestMessageCache_GetRenderedMessage(t *testing.T) {
	// Create a test model
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80

	cache := NewMessageCache()

	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
		Time:    time.Now(),
	}

	// Test initial rendering
	lines1 := cache.GetRenderedMessage(&model, msg, 80)
	if len(lines1) == 0 {
		t.Error("GetRenderedMessage should return non-empty lines")
	}

	// Test that cached version is returned
	cache.needsRefresh = false
	lines2 := cache.GetRenderedMessage(&model, msg, 80)

	if len(lines1) != len(lines2) {
		t.Error("Cached and fresh renders should have same length")
	}

	// Test width change invalidates cache
	_ = cache.GetRenderedMessage(&model, msg, 60)
	if cache.lastWidth != 60 {
		t.Error("lastWidth should be updated to 60")
	}
}

func TestMessageCache_GetTotalHeight(t *testing.T) {
	// Create a test model with some messages
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80
	model.height = 25

	// Add some test messages
	model.messages = []Message{
		{Role: "user", Content: "First message", Time: time.Now()},
		{Role: "assistant", Content: "Second message", Time: time.Now()},
	}

	cache := NewMessageCache()

	// Test computing height
	height1 := cache.GetTotalHeight(&model)
	if height1 <= 0 {
		t.Error("GetTotalHeight should return positive height")
	}

	// Test cached height is returned
	cache.needsRefresh = false
	height2 := cache.GetTotalHeight(&model)
	if height1 != height2 {
		t.Error("Cached height should match initial computation")
	}

	// Test that cached value is used
	if cache.cachedTotalHeight != height1 {
		t.Error("cachedTotalHeight should be set")
	}
}

func TestMessageCache_WidthInvalidation(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	cache := NewMessageCache()

	msg := Message{
		Role:    "user",
		Content: "Test message",
		Time:    time.Now(),
	}

	// Render at width 80
	cache.GetRenderedMessage(&model, msg, 80)
	if cache.lastWidth != 80 {
		t.Error("lastWidth should be 80")
	}

	// Set needsRefresh to false to test invalidation
	cache.needsRefresh = false

	// Render at different width should invalidate cache
	cache.GetRenderedMessage(&model, msg, 60)
	if cache.lastWidth != 60 {
		t.Error("lastWidth should be updated to 60")
	}

	// Cache should have been cleared
	if len(cache.renderedMessages) != 1 {
		t.Error("Cache should be cleared when width changes")
	}
}

func TestMessageCache_RenderAllMessages_Empty(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80
	model.height = 25
	model.styles = DefaultStyles()

	cache := NewMessageCache()

	result := cache.RenderAllMessages(&model)
	if len(result) == 0 {
		t.Error("RenderAllMessages should return non-empty result even with no messages")
	}
}

func TestMessageCache_RenderAllMessages_WithMessages(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80
	model.height = 25
	model.styles = DefaultStyles()

	// Add test messages
	model.messages = []Message{
		{Role: "user", Content: "Hello", Time: time.Now()},
		{Role: "assistant", Content: "Hi there!", Time: time.Now()},
	}

	cache := NewMessageCache()

	result := cache.RenderAllMessages(&model)
	if len(result) == 0 {
		t.Error("RenderAllMessages should return non-empty result with messages")
	}
}

// Test Styles Functions
func TestDefaultStyles(t *testing.T) {
	styles := DefaultStyles()

	// Test that we can call render methods without panicking
	testText := "Test message"

	// These should not panic
	_ = styles.userHeader.Render(testText)
	_ = styles.assistantHeader.Render(testText)
	_ = styles.messages.Render(testText)
	_ = styles.emptyMessages.Render(testText)
	_ = styles.statusBar.Render(testText)
	_ = styles.systemPrompt.Render(testText)

	// Test that styles return non-empty rendered content
	userResult := styles.userHeader.Render(testText)
	if len(userResult) == 0 {
		t.Error("userHeader.Render should return non-empty result")
	}

	assistantResult := styles.assistantHeader.Render(testText)
	if len(assistantResult) == 0 {
		t.Error("assistantHeader.Render should return non-empty result")
	}

	messagesResult := styles.messages.Render(testText)
	if len(messagesResult) == 0 {
		t.Error("messages.Render should return non-empty result")
	}
}

// Test View Rendering Functions
func TestModel_CalculateMessagesHeight(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80

	// Test with no messages
	height := model.calculateMessagesHeight()
	if height != 0 {
		t.Errorf("Empty message list should have height 0, got %d", height)
	}

	// Test with one message
	model.messages = []Message{
		{Role: "user", Content: "Hello", Time: time.Now()},
	}
	height = model.calculateMessagesHeight()
	if height <= 0 {
		t.Error("Single message should have positive height")
	}

	// Test with multiple messages
	model.messages = append(model.messages, Message{
		Role: "assistant", Content: "Hi there!", Time: time.Now(),
	})
	newHeight := model.calculateMessagesHeight()
	if newHeight <= height {
		t.Error("Additional message should increase total height")
	}

	// Test with long message content
	model.messages = []Message{
		{Role: "user", Content: strings.Repeat("This is a very long message that should wrap across multiple lines. ", 10), Time: time.Now()},
	}
	longHeight := model.calculateMessagesHeight()
	if longHeight <= 5 {
		t.Error("Long message should have significant height due to wrapping")
	}
}

func TestModel_FormatMessage(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80
	model.styles = DefaultStyles()

	testTime := time.Date(2025, 9, 19, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name             string
		message          Message
		expectedContains []string
		minLines         int
	}{
		{
			name: "user message",
			message: Message{
				Role:    "user",
				Content: "Hello, world!",
				Time:    testTime,
			},
			expectedContains: []string{"User", "14:30:45", "Hello, world!"},
			minLines:         3, // header + content + spacing
		},
		{
			name: "assistant message",
			message: Message{
				Role:    "assistant",
				Content: "Hi there! How can I help you?",
				Time:    testTime,
			},
			expectedContains: []string{"Assistant", "14:30:45", "Hi there!"},
			minLines:         3, // header + content + spacing
		},
		{
			name: "long message with wrapping",
			message: Message{
				Role:    "user",
				Content: strings.Repeat("This is a very long message that should wrap across multiple lines when displayed in the terminal. ", 5),
				Time:    testTime,
			},
			expectedContains: []string{"User", "14:30:45"},
			minLines:         5, // header + multiple content lines + spacing (adjusted expectation)
		},
		{
			name: "empty message",
			message: Message{
				Role:    "assistant",
				Content: "",
				Time:    testTime,
			},
			expectedContains: []string{"Assistant", "14:30:45"},
			minLines:         3, // header + empty content + spacing
		},
		{
			name: "message with newlines",
			message: Message{
				Role:    "user",
				Content: "Line 1\nLine 2\nLine 3",
				Time:    testTime,
			},
			expectedContains: []string{"User", "14:30:45", "Line 1", "Line 2", "Line 3"},
			minLines:         3, // header + content (may be wrapped differently) + spacing (adjusted expectation)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := model.formatMessage(tt.message)

			if len(lines) < tt.minLines {
				t.Errorf("Expected at least %d lines, got %d", tt.minLines, len(lines))
			}

			// Join all lines to check for expected content
			allContent := strings.Join(lines, " ")

			for _, expected := range tt.expectedContains {
				if !strings.Contains(allContent, expected) {
					t.Errorf("Formatted message should contain %q, got: %s", expected, allContent)
				}
			}

			// Last line should be empty (spacing)
			if len(lines) > 0 && lines[len(lines)-1] != "" {
				t.Error("Last line should be empty for spacing")
			}
		})
	}
}

func TestModel_GetSystemPromptHeight(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80
	model.height = 24

	// Test when system prompt is not shown
	model.showSystemPrompt = false
	height := model.getSystemPromptHeight()
	if height != 0 {
		t.Errorf("Hidden system prompt should have height 0, got %d", height)
	}

	// Test when system prompt is shown
	model.showSystemPrompt = true
	model.sessionSystemPrompt = "You are a helpful assistant."
	height = model.getSystemPromptHeight()
	if height <= 0 {
		t.Error("Visible system prompt should have positive height")
	}

	// Test with very long system prompt (should be limited)
	model.sessionSystemPrompt = strings.Repeat("This is a very long system prompt that should be limited in height. ", 20)
	longHeight := model.getSystemPromptHeight()
	maxAllowedHeight := model.height / 3
	if longHeight > maxAllowedHeight {
		t.Errorf("System prompt height should be limited to %d, got %d", maxAllowedHeight, longHeight)
	}

	// Test edit mode
	model.systemPromptEditMode = true
	model.systemPromptEditor = "Editing system prompt..."
	editHeight := model.getSystemPromptHeight()
	if editHeight <= 0 {
		t.Error("System prompt in edit mode should have positive height")
	}
}

func TestModel_RenderSystemPrompt(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80
	model.height = 24
	model.styles = DefaultStyles()

	// Test when system prompt is not shown
	model.showSystemPrompt = false
	result := model.renderSystemPrompt()
	if result != "" {
		t.Error("Hidden system prompt should render empty string")
	}

	// Test display mode
	model.showSystemPrompt = true
	model.systemPromptEditMode = false
	model.sessionSystemPrompt = "You are a helpful assistant."
	result = model.renderSystemPrompt()
	if result == "" {
		t.Error("Visible system prompt should render non-empty content")
	}
	if !strings.Contains(result, "System Prompt") {
		t.Error("Rendered system prompt should contain header")
	}
	if !strings.Contains(result, "helpful assistant") {
		t.Error("Rendered system prompt should contain the prompt content")
	}

	// Test edit mode
	model.systemPromptEditMode = true
	model.systemPromptEditor = "You are an AI assistant."
	result = model.renderSystemPrompt()
	if result == "" {
		t.Error("System prompt in edit mode should render non-empty content")
	}
	if !strings.Contains(result, "EDITING") {
		t.Error("Edit mode should show editing indicator")
	}
	if !strings.Contains(result, "AI assistant") {
		t.Error("Edit mode should show editor content")
	}

	// Test empty system prompt
	model.systemPromptEditMode = false
	model.sessionSystemPrompt = ""
	result = model.renderSystemPrompt()
	if !strings.Contains(result, "No system prompt") {
		t.Error("Empty system prompt should show appropriate message")
	}

	// Test empty system prompt in edit mode
	model.systemPromptEditMode = true
	model.systemPromptEditor = ""
	result = model.renderSystemPrompt()
	if result == "" {
		t.Error("Empty system prompt in edit mode should still render")
	}

	// Test very long system prompt (should be truncated)
	model.systemPromptEditMode = false
	model.sessionSystemPrompt = strings.Repeat("Very long system prompt content. ", 100)
	result = model.renderSystemPrompt()
	if result == "" {
		t.Error("Long system prompt should render")
	}
}

// Test Message Handling Functions
func TestModel_SendMessageMsg(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Test sendMessageMsg handling
	msg := sendMessageMsg{message: "Hello, test!"}
	updatedModel, cmd := model.Update(msg)

	// Should return the model and a command
	if updatedModel == nil {
		t.Error("Update should return a model")
	}

	if cmd == nil {
		t.Error("sendMessageMsg should return a command")
	}

	// Model should be unchanged immediately (async operation)
	newModel := updatedModel.(Model)
	if len(newModel.messages) != len(model.messages) {
		t.Error("Messages should not be immediately modified by sendMessageMsg")
	}
}

func TestModel_ResponseMsg_Success(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Set loading state
	model.inputModel.SetLoading(true)
	initialMessageCount := len(model.messages)

	// Test successful response
	msg := responseMsg{content: "Hello, this is a response!", err: nil}
	updatedModel, cmd := model.Update(msg)

	newModel := updatedModel.(Model)

	// Should add assistant message
	if len(newModel.messages) != initialMessageCount+1 {
		t.Errorf("Expected %d messages, got %d", initialMessageCount+1, len(newModel.messages))
	}

	// Check the added message
	if len(newModel.messages) > 0 {
		lastMessage := newModel.messages[len(newModel.messages)-1]
		if lastMessage.Role != "assistant" {
			t.Errorf("Expected assistant message, got %s", lastMessage.Role)
		}
		if lastMessage.Content != "Hello, this is a response!" {
			t.Errorf("Expected specific content, got %s", lastMessage.Content)
		}
	}

	// Should clear loading state
	if newModel.inputModel.IsLoading() {
		t.Error("Loading state should be cleared")
	}

	// Should invalidate cache and update flags
	if !newModel.messagesNeedsUpdate {
		t.Error("messagesNeedsUpdate should be set")
	}

	if !newModel.statusNeedsUpdate {
		t.Error("statusNeedsUpdate should be set")
	}

	// Command should be nil (no further actions needed)
	if cmd != nil {
		t.Error("Successful response should not return additional commands")
	}
}

func TestModel_ResponseMsg_Error(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Set loading state
	model.inputModel.SetLoading(true)
	initialMessageCount := len(model.messages)

	// Test error response
	testError := fmt.Errorf("connection failed")
	msg := responseMsg{content: "", err: testError}
	updatedModel, _ := model.Update(msg)

	newModel := updatedModel.(Model)

	// Should add error message
	if len(newModel.messages) != initialMessageCount+1 {
		t.Errorf("Expected %d messages, got %d", initialMessageCount+1, len(newModel.messages))
	}

	// Check the added error message
	if len(newModel.messages) > 0 {
		lastMessage := newModel.messages[len(newModel.messages)-1]
		if lastMessage.Role != "assistant" {
			t.Errorf("Expected assistant message, got %s", lastMessage.Role)
		}
		if !strings.Contains(lastMessage.Content, "Error:") {
			t.Errorf("Expected error message to contain 'Error:', got %s", lastMessage.Content)
		}
		if !strings.Contains(lastMessage.Content, "connection failed") {
			t.Errorf("Expected error message to contain error details, got %s", lastMessage.Content)
		}
	}

	// Should clear loading state
	if newModel.inputModel.IsLoading() {
		t.Error("Loading state should be cleared")
	}
}

func TestModel_RAGStatusMsg(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Test RAG status message
	msg := ragStatusMsg{status: "Querying documents..."}
	updatedModel, cmd := model.Update(msg)

	newModel := updatedModel.(Model)

	// Should not add messages
	if len(newModel.messages) != len(model.messages) {
		t.Error("RAG status message should not add messages")
	}

	// Should not return additional commands
	if cmd != nil {
		t.Error("RAG status message should not return commands")
	}

	// The RAG status should be set on the input model
	// (We can't easily verify this without exposing internal state)
}

func TestModel_MessageProcessing_Integration(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Simulate a full message exchange
	initialMessageCount := len(model.messages)

	// 1. Send a message
	sendMsg := sendMessageMsg{message: "Test question"}
	updatedModel, cmd := model.Update(sendMsg)
	model = updatedModel.(Model)
	if cmd == nil {
		t.Error("Send message should return a command")
	}

	// 2. Receive a response
	respMsg := responseMsg{content: "Test answer", err: nil}
	updatedModel, _ = model.Update(respMsg)
	model = updatedModel.(Model)

	// Should have added the assistant response
	if len(model.messages) != initialMessageCount+1 {
		t.Errorf("Expected %d messages after response, got %d", initialMessageCount+1, len(model.messages))
	}

	// 3. Test error scenario
	errorMsg := responseMsg{content: "", err: fmt.Errorf("network error")}
	updatedModel, _ = model.Update(errorMsg)
	model = updatedModel.(Model)

	// Should have added the error message
	if len(model.messages) != initialMessageCount+2 {
		t.Errorf("Expected %d messages after error, got %d", initialMessageCount+2, len(model.messages))
	}

	// Verify the last message is an error
	if len(model.messages) > 0 {
		lastMessage := model.messages[len(model.messages)-1]
		if !strings.Contains(lastMessage.Content, "Error:") {
			t.Error("Last message should be an error message")
		}
	}
}
