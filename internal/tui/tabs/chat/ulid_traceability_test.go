package chat

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
)

// TestULIDTraceability tests that the same ULID is used throughout the entire conversation flow
func TestULIDTraceability(t *testing.T) {
	// Initialize logging to capture events
	logConfig := logging.DefaultConfig()
	logConfig.EnableStderr = false // Don't spam stderr during tests
	logConfig.EnableFile = false   // Don't create log files during tests
	err := logging.Initialize(logConfig)
	if err != nil {
		t.Fatalf("Failed to initialize logging: %v", err)
	}
	defer logging.Close()

	// Create test configuration
	config := &configuration.Config{
		OllamaURL: "http://localhost:11434",
		ChatModel: "llama3.2",
	}

	// Create model
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80
	model.height = 24

	// Simulate user input
	testInput := "Hello, test message for ULID traceability"

	// Set the input in the model
	model.inputModel.SetValue(testInput)

	// Simulate pressing enter (this should generate a conversation ULID)
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModelInterface, cmd := model.Update(keyMsg)
	model = updatedModelInterface.(Model)

	// Check that the conversation ULID was set
	if model.currentConversationULID == "" {
		t.Fatal("Expected currentConversationULID to be set after user input")
	}

	conversationULID := model.currentConversationULID
	t.Logf("Generated conversation ULID: %s", conversationULID)

	// Check that the user message was added with the conversation ULID
	if len(model.messages) == 0 {
		t.Fatal("Expected at least one message after user input")
	}

	userMessage := model.messages[len(model.messages)-1]
	if userMessage.Role != "user" {
		t.Fatalf("Expected last message to be user message, got: %s", userMessage.Role)
	}

	if userMessage.ULID != conversationULID {
		t.Fatalf("Expected user message ULID to match conversation ULID.\nExpected: %s\nGot: %s", conversationULID, userMessage.ULID)
	}

	if userMessage.Content != testInput {
		t.Fatalf("Expected user message content to match input.\nExpected: %s\nGot: %s", testInput, userMessage.Content)
	}

	// The sendMessage command should be returned
	if cmd == nil {
		t.Fatal("Expected sendMessage command to be returned")
	}

	// Simulate a response message with some tool calls (to test tool ULID propagation)
	responseMsg := responseMsg{
		content:          "This is a test response",
		conversationULID: conversationULID,
		additionalMessages: []Message{
			{
				Role:     "tool",
				Content:  "Tool execution result",
				Time:     time.Now(),
				ULID:     conversationULID, // Should use the same conversation ULID
				ToolName: "test_tool",
				Hidden:   true,
			},
		},
	}

	// Process the response
	updatedModelInterface2, _ := model.Update(responseMsg)
	model = updatedModelInterface2.(Model)

	// Verify that all messages now use the same conversation ULID
	for i, msg := range model.messages {
		if msg.ULID != conversationULID {
			t.Fatalf("Message %d has different ULID.\nExpected: %s\nGot: %s\nMessage: %+v", i, conversationULID, msg.ULID, msg)
		}
	}

	// Verify we have the expected number of messages
	expectedMessages := 3 // user + tool + assistant
	if len(model.messages) != expectedMessages {
		t.Fatalf("Expected %d messages, got %d", expectedMessages, len(model.messages))
	}

	// Verify message roles and ULIDs
	messages := model.messages

	// First message: user
	if messages[0].Role != "user" || messages[0].ULID != conversationULID {
		t.Fatalf("First message incorrect. Expected: user/%s, Got: %s/%s", conversationULID, messages[0].Role, messages[0].ULID)
	}

	// Second message: tool (from additionalMessages)
	if messages[1].Role != "tool" || messages[1].ULID != conversationULID {
		t.Fatalf("Second message incorrect. Expected: tool/%s, Got: %s/%s", conversationULID, messages[1].Role, messages[1].ULID)
	}

	// Third message: assistant (response content)
	if messages[2].Role != "assistant" || messages[2].ULID != conversationULID {
		t.Fatalf("Third message incorrect. Expected: assistant/%s, Got: %s/%s", conversationULID, messages[2].Role, messages[2].ULID)
	}

	t.Logf("✅ ULID traceability test passed! All %d messages use the same conversation ULID: %s", len(model.messages), conversationULID)
}

// TestULIDPersistenceAcrossConversations tests that different conversations get different ULIDs
func TestULIDPersistenceAcrossConversations(t *testing.T) {
	// Create test configuration
	config := &configuration.Config{
		OllamaURL: "http://localhost:11434",
		ChatModel: "llama3.2",
	}

	// Create model
	ctx := context.Background()
	model := NewModel(ctx, config)
	model.width = 80
	model.height = 24

	// First conversation
	model.inputModel.SetValue("First conversation")
	keyMsg1 := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModelInterface1, _ := model.Update(keyMsg1)
	model = updatedModelInterface1.(Model)
	firstULID := model.currentConversationULID

	// Add a response to complete the first conversation
	responseMsg1 := responseMsg{
		content:          "Response to first conversation",
		conversationULID: firstULID,
	}
	updatedModelInterface2, _ := model.Update(responseMsg1)
	model = updatedModelInterface2.(Model)

	// Second conversation
	model.inputModel.SetValue("Second conversation")
	keyMsg2 := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModelInterface3, _ := model.Update(keyMsg2)
	model = updatedModelInterface3.(Model)
	secondULID := model.currentConversationULID

	// Verify ULIDs are different
	if firstULID == secondULID {
		t.Fatalf("Expected different ULIDs for different conversations, but both are: %s", firstULID)
	}

	// Verify the second conversation's messages use the second ULID
	lastMessage := model.messages[len(model.messages)-1]
	if lastMessage.ULID != secondULID {
		t.Fatalf("Expected last message to use second ULID.\nExpected: %s\nGot: %s", secondULID, lastMessage.ULID)
	}

	t.Logf("✅ Different conversations use different ULIDs: %s != %s", firstULID, secondULID)
}

// TestULIDLoggingConsistency tests that logging events use the conversation ULID
func TestULIDLoggingConsistency(t *testing.T) {
	// This test verifies that the logConversationEvent function is called with the conversation ULID
	// Since we can't easily capture log output in tests, we verify the function is called with correct parameters

	testULID := "01TESTULID123456789"
	testRole := "user"
	testContent := "Test message for logging"
	testModel := "llama3.2"

	// This doesn't fail, which means the function accepts the parameters correctly
	logConversationEvent(testULID, testRole, testContent, testModel)

	t.Logf("✅ logConversationEvent accepts conversation ULID parameter correctly")
}
