package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestModel_EmptyResponseHandling(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	initialMessageCount := len(model.messages)

	// Test responseMsg with empty content (simulating LLM returning empty response)
	msg := responseMsg{
		content:            "", // Empty content from LLM
		err:                nil,
		additionalMessages: nil, // No additional messages (no tool calls)
		conversationULID:   "test-ulid-123",
	}

	updatedModel, cmd := model.Update(msg)
	newModel := updatedModel.(Model)

	// The response should be handled gracefully with a helpful message
	if len(newModel.messages) != initialMessageCount+1 {
		t.Errorf("Expected %d messages after empty response, got %d", initialMessageCount+1, len(newModel.messages))
	}

	// Check that a helpful message was added instead of empty content
	if len(newModel.messages) > 0 {
		lastMessage := newModel.messages[len(newModel.messages)-1]
		if lastMessage.Role != "assistant" {
			t.Error("Last message should be from assistant")
		}
		if lastMessage.Content == "" {
			t.Error("Empty response should be replaced with helpful message")
		}
		if !strings.Contains(lastMessage.Content, "unable to provide a response") {
			t.Error("Message should explain inability to respond")
		}
	}

	// Check that loading state was turned off
	if newModel.inputModel.IsLoading() {
		t.Error("Loading state should be turned off after response")
	}

	if cmd != nil {
		t.Error("No command should be returned for empty response")
	}
}

func TestModel_WhitespaceOnlyResponseHandling(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	initialMessageCount := len(model.messages)

	// Test responseMsg with whitespace-only content
	msg := responseMsg{
		content:            "   \n\t  \n  ", // Whitespace-only content
		err:                nil,
		additionalMessages: nil,
		conversationULID:   "test-ulid-456",
	}

	updatedModel, cmd := model.Update(msg)
	newModel := updatedModel.(Model)

	// Whitespace-only responses should be handled gracefully
	if len(newModel.messages) != initialMessageCount+1 {
		t.Errorf("Expected %d messages after whitespace response, got %d", initialMessageCount+1, len(newModel.messages))
	}

	// Check that a helpful message was added instead of whitespace content
	if len(newModel.messages) > 0 {
		lastMessage := newModel.messages[len(newModel.messages)-1]
		if lastMessage.Role != "assistant" {
			t.Error("Last message should be from assistant")
		}
		if strings.TrimSpace(lastMessage.Content) == "" {
			t.Error("Whitespace response should be replaced with helpful message")
		}
		if !strings.Contains(lastMessage.Content, "unable to provide a response") {
			t.Error("Message should explain inability to respond")
		}
	}

	if cmd != nil {
		t.Error("No command should be returned for whitespace-only response")
	}
}
