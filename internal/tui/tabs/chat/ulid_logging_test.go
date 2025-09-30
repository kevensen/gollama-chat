package chat

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestULIDGeneration(t *testing.T) {
	// Test that generateULID creates valid ULIDs
	ulid1 := generateULID()
	ulid2 := generateULID()

	// ULIDs should be 26 characters long
	if len(ulid1) != 26 {
		t.Errorf("Expected ULID length of 26, got %d for ULID: %s", len(ulid1), ulid1)
	}

	if len(ulid2) != 26 {
		t.Errorf("Expected ULID length of 26, got %d for ULID: %s", len(ulid2), ulid2)
	}

	// ULIDs should be different
	if ulid1 == ulid2 {
		t.Errorf("Expected different ULIDs, but got the same: %s", ulid1)
	}

	// ULIDs should be sortable by time (first generated should be lexicographically smaller)
	if ulid1 >= ulid2 {
		t.Errorf("Expected ULID1 (%s) to be lexicographically smaller than ULID2 (%s)", ulid1, ulid2)
	}

	t.Logf("Generated ULID1: %s", ulid1)
	t.Logf("Generated ULID2: %s", ulid2)
}

func TestMessageULIDAssignment(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := context.Background()
	model := NewModel(ctx, config)

	// Test that user messages get ULIDs assigned
	model.inputModel.SetValue("Test message")

	// Simulate enter key press to add user message
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(keyMsg)
	newModel := updatedModel.(Model)

	// Check that a message was added and has a ULID
	if len(newModel.messages) == 0 {
		t.Error("Expected a message to be added")
		return
	}

	lastMessage := newModel.messages[len(newModel.messages)-1]
	if lastMessage.ULID == "" {
		t.Error("Expected message to have a ULID assigned")
	}

	if len(lastMessage.ULID) != 26 {
		t.Errorf("Expected ULID length of 26, got %d", len(lastMessage.ULID))
	}

	t.Logf("Message ULID: %s", lastMessage.ULID)
	t.Logf("Message Content: %s", lastMessage.Content)
	t.Logf("Message Role: %s", lastMessage.Role)
}

func TestConversationLogging(t *testing.T) {
	// Test the logging function doesn't panic
	messageID := generateULID()
	role := "user"
	content := "Test message for logging"
	model := "test-model"

	// This should not panic
	logConversationEvent(messageID, role, content, model)

	t.Logf("Successfully logged conversation event with ULID: %s", messageID)
}

func TestContentPreview(t *testing.T) {
	tests := []struct {
		content   string
		maxLength int
		expected  string
	}{
		{"Short message", 100, "Short message"},
		{"This is a long message that should be truncated", 20, "This is a long messa..."},
		{"", 10, ""},
		{"Exact", 5, "Exact"},
		{"Too long", 5, "Too l..."},
	}

	for _, test := range tests {
		result := getContentPreview(test.content, test.maxLength)
		if result != test.expected {
			t.Errorf("Expected preview %q, got %q for content %q with maxLength %d",
				test.expected, result, test.content, test.maxLength)
		}
	}
}
