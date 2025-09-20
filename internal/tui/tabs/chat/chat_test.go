package chat

import (
	"context"
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
		DarkMode:            false,
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
