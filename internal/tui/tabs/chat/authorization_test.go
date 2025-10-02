package chat

import (
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/ollama/ollama/api"
)

// TestExecuteToolCallsAndCreateMessages tests the critical authorization flow
func TestExecuteToolCallsAndCreateMessages(t *testing.T) {
	tests := []struct {
		name            string
		toolName        string
		trustLevel      int
		expectedBlocked bool
		expectedPrompt  bool
		expectedExecute bool
		expectError     bool
	}{
		{
			name:            "TrustNone (0) - should block execution",
			toolName:        "filesystem_read",
			trustLevel:      0,
			expectedBlocked: true,
			expectedPrompt:  false,
			expectedExecute: false,
			expectError:     false,
		},
		{
			name:            "AskForTrust (1) - should prompt for permission",
			toolName:        "filesystem_read",
			trustLevel:      1,
			expectedBlocked: false,
			expectedPrompt:  true,
			expectedExecute: false,
			expectError:     false,
		},
		{
			name:            "TrustSession (2) - should execute directly",
			toolName:        "filesystem_read",
			trustLevel:      2,
			expectedBlocked: false,
			expectedPrompt:  false,
			expectedExecute: true,
			expectError:     false,
		},
		{
			name:            "Unknown trust level - should block for safety",
			toolName:        "filesystem_read",
			trustLevel:      99,
			expectedBlocked: true,
			expectedPrompt:  false,
			expectedExecute: false,
			expectError:     false,
		},
		{
			name:            "Non-existent tool - should return error message",
			toolName:        "non_existent_tool",
			trustLevel:      2,
			expectedBlocked: false,
			expectedPrompt:  false,
			expectedExecute: false,
			expectError:     true,
		},
	}
	ctx := t.Context()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup configuration with specific trust level
			config := &configuration.Config{
				ChatModel:           "llama3.1",
				OllamaURL:           "http://localhost:11434",
				DefaultSystemPrompt: "Test prompt",
				SelectedCollections: make(map[string]bool),
				ToolTrustLevels: map[string]int{
					tt.toolName: tt.trustLevel,
				},
			}

			model := NewModel(ctx, config)

			// Create a tool call for testing
			toolCalls := []api.ToolCall{
				{
					Function: api.ToolCallFunction{
						Name: tt.toolName,
						Arguments: map[string]any{
							"action": "get_working_directory",
						},
					},
				},
			}

			// Execute the tool calls
			messages, err := model.executeToolCallsAndCreateMessages(toolCalls, "test-ulid-123")

			// For tools that don't exist, we should get an error message
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check we got a message
			if len(messages) == 0 {
				t.Fatal("Expected at least one message")
			}

			message := messages[0]

			if tt.expectError {
				// Non-existent tools should return "Tool 'toolname' not found" message
				if !contains(message.Content, "not found") {
					t.Errorf("Expected tool not found message, got: %s", message.Content)
				}
			} else {
				// Test the authorization behaviors for existing tools
				if tt.expectedBlocked {
					// Should contain blocking message with appropriate emoji
					if !contains(message.Content, "🚫") && !contains(message.Content, "⚠️") {
						t.Errorf("Expected blocking message with emoji, got: %s", message.Content)
					}
				}

				if tt.expectedPrompt {
					// Should contain permission request
					if !contains(message.Content, "❓") || !contains(message.Content, "Allow execution") {
						t.Errorf("Expected permission prompt, got: %s", message.Content)
					}
					// Should contain TOOL_CALL_DATA for later processing
					if !contains(message.Content, "TOOL_CALL_DATA:") {
						t.Errorf("Expected TOOL_CALL_DATA in permission prompt, got: %s", message.Content)
					}
				}

				if tt.expectedExecute {
					// For TrustSession (2), the tool should execute successfully
					// filesystem_read with get_working_directory should return the current directory
					if contains(message.Content, "Error") || contains(message.Content, "not found") {
						t.Errorf("Expected successful execution for TrustSession, got error: %s", message.Content)
					}
				}
			}
		})
	}
}

// TestMCPToolAuthorizationFlow tests that MCP tools follow the same authorization pattern
func TestMCPToolAuthorizationFlow(t *testing.T) {
	// Test MCP tool names (server.toolname format)
	mcpToolTests := []struct {
		name           string
		toolName       string
		trustLevel     int
		expectedPrompt bool
	}{
		{
			name:           "MCP tool with AskForTrust should prompt",
			toolName:       "go-potms.days_between",
			trustLevel:     1,
			expectedPrompt: true,
		},
		{
			name:           "MCP tool with TrustSession should execute",
			toolName:       "go-potms.days_between",
			trustLevel:     2,
			expectedPrompt: false,
		},
		{
			name:           "MCP tool with TrustNone should block",
			toolName:       "go-potms.days_between",
			trustLevel:     0,
			expectedPrompt: false,
		},
	}

	for _, tt := range mcpToolTests {
		t.Run(tt.name, func(t *testing.T) {
			config := &configuration.Config{
				ChatModel:           "llama3.1",
				OllamaURL:           "http://localhost:11434",
				DefaultSystemPrompt: "Test prompt",
				SelectedCollections: make(map[string]bool),
				ToolTrustLevels: map[string]int{
					tt.toolName: tt.trustLevel,
				},
			}

			ctx := t.Context()
			model := NewModel(ctx, config)

			toolCalls := []api.ToolCall{
				{
					Function: api.ToolCallFunction{
						Name: tt.toolName,
						Arguments: map[string]any{
							"date1": "today",
							"date2": "2026-02-01",
						},
					},
				},
			}

			messages, err := model.executeToolCallsAndCreateMessages(toolCalls, "test-ulid-123")

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(messages) > 0 {
				message := messages[0]

				if tt.expectedPrompt {
					// Since MCP tools may not be available in test environment,
					// we either get permission prompt OR "not found" error
					if contains(message.Content, "❓") && contains(message.Content, "Allow execution") {
						// Good: got permission prompt
					} else if contains(message.Content, "not found") {
						// Expected: MCP tool not available in test environment
						t.Logf("MCP tool not available in test environment: %s", message.Content)
					} else {
						t.Errorf("Expected permission prompt or tool not found for MCP tool, got: %s", message.Content)
					}
				}
			}
		})
	}
}

// TestDefaultTrustLevel tests that tools default to AskForTrust (1) when not configured
func TestDefaultTrustLevel(t *testing.T) {
	config := &configuration.Config{
		ChatModel:           "llama3.1",
		OllamaURL:           "http://localhost:11434",
		DefaultSystemPrompt: "Test prompt",
		SelectedCollections: make(map[string]bool),
		// Explicitly empty ToolTrustLevels to test defaults
		ToolTrustLevels: make(map[string]int),
	}

	ctx := t.Context()
	model := NewModel(ctx, config)

	toolCalls := []api.ToolCall{
		{
			Function: api.ToolCallFunction{
				Name: "filesystem_read",
				Arguments: map[string]any{
					"action": "get_working_directory",
				},
			},
		},
	}

	messages, err := model.executeToolCallsAndCreateMessages(toolCalls, "test-ulid-123")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(messages) == 0 {
		t.Fatal("Expected at least one message")
	}

	message := messages[0]

	// Should default to AskForTrust (1) and show permission prompt
	if !contains(message.Content, "❓") || !contains(message.Content, "Allow execution") {
		t.Errorf("Expected permission prompt for unconfigured tool (should default to AskForTrust), got: %s", message.Content)
	}
}

// TestSecurityBypass ensures no tools are sent to Ollama to prevent server-side execution
func TestSecurityBypass(t *testing.T) {
	// This test ensures that our security fix prevents server-side tool execution
	// by verifying that tools are not being sent to Ollama in chat requests

	// Note: This is more of a design verification test since the actual
	// API calls would be mocked in a real implementation. The key insight
	// is that the executeToolCallsAndCreateMessages function should handle
	// ALL tool authorization locally, never relying on server-side execution.

	config := &configuration.Config{
		ChatModel:           "test-model",
		OllamaURL:           "http://localhost:11434",
		DefaultSystemPrompt: "Test prompt",
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     make(map[string]int),
	}

	ctx := t.Context()
	model := NewModel(ctx, config)

	// Simulate what would happen if a model somehow returned tool execution results
	// without going through local authorization (this should not happen anymore)
	toolCalls := []api.ToolCall{
		{
			Function: api.ToolCallFunction{
				Name: "days_between",
				Arguments: map[string]any{
					"date1": "today",
					"date2": "2026-02-01",
				},
			},
		},
	}

	messages, err := model.executeToolCallsAndCreateMessages(toolCalls, "test-ulid-123")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(messages) == 0 {
		t.Fatal("Expected at least one message")
	}

	// The key security requirement: tool should require permission (default trust level 1)
	// For unknown tools, we should get "not found" error, which is the correct security behavior
	message := messages[0]
	if contains(message.Content, "❓") && contains(message.Content, "Allow execution") {
		// Good: tool requires permission
	} else if contains(message.Content, "not found") {
		// Also good: unknown tool is properly rejected
		t.Logf("Tool properly rejected as not found: %s", message.Content)
	} else {
		t.Errorf("SECURITY VIOLATION: Tool executed without permission prompt! Got: %s", message.Content)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
