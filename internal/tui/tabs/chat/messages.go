package chat

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ollama/ollama/api"

	"github.com/kevensen/gollama-chat/internal/tooling"
)

// sendMessage sends a message to Ollama using the Ollama API client
func (m Model) sendMessage(prompt string, conversationULID string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		var fullPrompt string

		// If RAG is enabled, use it to retrieve relevant documents
		if m.config.RAGEnabled && m.ragService != nil && m.ragService.IsReady() {
			ragResult, err := m.ragService.QueryDocuments(m.ctx, prompt)
			if err == nil && ragResult != nil && len(ragResult.Documents) > 0 {
				// Add formatted RAG documents to the prompt
				fullPrompt = ragResult.FormatDocumentsForPrompt() + prompt
			} else {
				fullPrompt = prompt
			}
		} else {
			fullPrompt = prompt
		}

		// Create Ollama client with the configured URL
		baseURL, err := url.Parse(m.config.OllamaURL)
		if err != nil {
			return responseMsg{err: fmt.Errorf("invalid Ollama URL %s: %w", m.config.OllamaURL, err)}
		}
		client := api.NewClient(baseURL, &http.Client{
			Timeout: 60 * time.Second, // Add a reasonable timeout
		})

		// Convert chat messages to Ollama api.Message format
		var messages []api.Message

		// Add system prompt if configured
		if m.sessionSystemPrompt != "" {
			messages = append(messages, api.Message{
				Role:    "system",
				Content: m.sessionSystemPrompt,
			})
		}

		// Add all previous messages as context (preserving message history)
		// Note: The current user message is already in m.messages, so we need to
		// replace the last message with the RAG-enhanced version if RAG is enabled
		for i, msg := range m.messages {
			if i == len(m.messages)-1 && msg.Role == "user" && fullPrompt != prompt {
				// This is the last message and RAG enhanced the prompt, use the enhanced version
				messages = append(messages, api.Message{
					Role:    msg.Role,
					Content: fullPrompt,
				})
			} else {
				// Convert chat.Message to api.Message with proper tool handling
				apiMsg := api.Message{
					Role:    msg.Role,
					Content: msg.Content,
				}

				// For tool messages, set the ToolName field
				if msg.Role == "tool" && msg.ToolName != "" {
					apiMsg.ToolName = msg.ToolName
					// Clean up the content to remove the [Tool name]: prefix for API
					if strings.HasPrefix(msg.Content, "[Tool "+msg.ToolName+"]: ") {
						apiMsg.Content = strings.TrimPrefix(msg.Content, "[Tool "+msg.ToolName+"]: ")
					}
				}

				// For assistant messages, restore ToolCalls if present
				if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
					var toolCalls []api.ToolCall
					for _, tcInfo := range msg.ToolCalls {
						toolCall := api.ToolCall{
							Function: api.ToolCallFunction{
								Name:      tcInfo.FunctionName,
								Arguments: tcInfo.Arguments,
							},
						}
						toolCalls = append(toolCalls, toolCall)
					}
					apiMsg.ToolCalls = toolCalls
				}

				messages = append(messages, apiMsg)
			}
		}

		// Set options
		options := map[string]any{
			"temperature":    0.7,
			"repeat_last_n":  2,
			"repeat_penalty": 1.1,
		}

		// SECURITY FIX: Do not send tools to Ollama to prevent server-side execution
		// that bypasses local authorization. All tool calls must go through local
		// authorization as per AGENTS.md requirements.
		//
		// Previous code sent tools directly to Ollama, allowing models to execute
		// tools server-side without user consent, violating the authorization requirements.

		// Create chat request with stream enabled (true is default, but we're explicit)
		stream := true
		chatRequest := &api.ChatRequest{
			Model:    m.config.ChatModel,
			Messages: messages,
			Stream:   &stream,
			Options:  options,
			// Tools:    nil, // Explicitly do not send tools to prevent unauthorized execution
		}

		// Use ChatStream for real-time response with enhanced error handling
		var fullResponse strings.Builder
		var responseErr error
		var toolCalls []api.ToolCall

		err = client.Chat(m.ctx, chatRequest, func(response api.ChatResponse) error {
			// Check for context cancellation
			if m.ctx.Err() != nil {
				responseErr = m.ctx.Err()
				return responseErr
			}

			// Accumulate the text response
			fullResponse.WriteString(response.Message.Content)

			// Collect tool calls if present
			if len(response.Message.ToolCalls) > 0 {
				toolCalls = append(toolCalls, response.Message.ToolCalls...)
			}

			return nil
		})

		if err != nil {
			if responseErr != nil {
				return responseMsg{err: fmt.Errorf("chat response error: %w", responseErr)}
			}
			return responseMsg{err: fmt.Errorf("chat request failed: %w", err)}
		}

		// Execute tool calls if present and get follow-up response
		responseContent := fullResponse.String()
		var additionalMessages []Message

		if len(toolCalls) > 0 {
			// Execute the tool calls
			toolResultMessages, toolErr := m.executeToolCallsAndCreateMessages(toolCalls, conversationULID)
			if toolErr != nil {
				responseContent += fmt.Sprintf("\n\n[Tool execution error: %v]", toolErr)
			} else {
				// Only add assistant message with tool calls if it has content
				if strings.TrimSpace(responseContent) != "" {
					// Convert api.ToolCall to ToolCallInfo for storage
					var toolCallInfos []ToolCallInfo
					for _, tc := range toolCalls {
						toolCallInfos = append(toolCallInfos, ToolCallInfo{
							FunctionName: tc.Function.Name,
							Arguments:    tc.Function.Arguments,
						})
					}

					assistantWithToolsMsg := Message{
						Role:      "assistant",
						Content:   responseContent,
						Time:      time.Now(),
						ULID:      conversationULID, // Use conversation ULID for traceability
						ToolCalls: toolCallInfos,
					}
					additionalMessages = append(additionalMessages, assistantWithToolsMsg)
				}

				// Convert tool result api.Messages to chat.Messages and add to history
				for _, toolResultMsg := range toolResultMessages {
					chatToolMsg := Message{
						Role:     "tool",                // Use "tool" role to distinguish from regular messages
						Content:  toolResultMsg.Content, // Store clean content without prefix
						Time:     time.Now(),
						ULID:     conversationULID, // Use conversation ULID for traceability
						ToolName: toolResultMsg.ToolName,
						Hidden:   true, // Hide tool messages from TUI display
					}
					additionalMessages = append(additionalMessages, chatToolMsg)
				}

				// Add assistant message with tool calls to messages for follow-up API call
				messages = append(messages, api.Message{
					Role:      "assistant",
					Content:   responseContent,
					ToolCalls: toolCalls,
				})

				// Add tool result messages for follow-up API call
				messages = append(messages, toolResultMessages...)

				// Make another API call to get the LLM's response to the tool results
				followUpRequest := &api.ChatRequest{
					Model:    m.config.ChatModel,
					Messages: messages,
					Stream:   &stream,
					Options:  options,
					// Tools:    nil, // Explicitly do not send tools to prevent unauthorized execution
				}

				var followUpResponse strings.Builder
				followUpErr := client.Chat(m.ctx, followUpRequest, func(response api.ChatResponse) error {
					if m.ctx.Err() != nil {
						return m.ctx.Err()
					}
					followUpResponse.WriteString(response.Message.Content)
					return nil
				})

				if followUpErr != nil {
					responseContent += fmt.Sprintf("\n\n[Follow-up response error: %v]", followUpErr)
				} else {
					responseContent = followUpResponse.String()
				}
			}
		}

		return responseMsg{
			content:            responseContent,
			additionalMessages: additionalMessages,
			conversationULID:   conversationULID,
		}
	})
}

// executeToolCallsAndCreateMessages executes the tool calls and returns the tool result messages
func (m Model) executeToolCallsAndCreateMessages(toolCalls []api.ToolCall, conversationULID string) ([]api.Message, error) {
	var messages []api.Message

	for _, toolCall := range toolCalls {
		// Get the tool from the registry (unified tool that supports both builtin and MCP)
		tool, exists := tooling.DefaultRegistry.GetUnifiedTool(toolCall.Function.Name)
		if !exists {
			// Create error message for unknown tool
			messages = append(messages, api.Message{
				Role:     "tool",
				Content:  fmt.Sprintf("Error: Tool '%s' not found", toolCall.Function.Name),
				ToolName: toolCall.Function.Name,
			})
			continue
		}

		// Check if tool is available (especially important for MCP tools)
		if !tool.Available {
			messages = append(messages, api.Message{
				Role:     "tool",
				Content:  fmt.Sprintf("Error: Tool '%s' is not available (server may be down)", toolCall.Function.Name),
				ToolName: toolCall.Function.Name,
			})
			continue
		}

		// Check tool trust level from configuration
		trustLevel := m.config.GetToolTrustLevel(toolCall.Function.Name)

		// Handle trust levels
		switch trustLevel {
		case 0: // TrustNone - block execution
			messages = append(messages, api.Message{
				Role:     "tool",
				Content:  fmt.Sprintf("🚫 Tool '%s' execution blocked: Tool trust is set to 'None'. Go to Tools tab (press 't') to change trust level to 'Session' to allow execution.", toolCall.Function.Name),
				ToolName: toolCall.Function.Name,
			})
			continue
		case 1: // AskForTrust - require user permission
			// Create a permission request message that will be handled by the UI
			permissionMsg := fmt.Sprintf("❓ Tool '%s' wants to execute with arguments: %v\n\nAllow execution? (y)es / (n)o / (t)rust for session\n\nTOOL_CALL_DATA:%s:%v",
				toolCall.Function.Name, toolCall.Function.Arguments, toolCall.Function.Name, toolCall.Function.Arguments)

			messages = append(messages, api.Message{
				Role:     "tool",
				Content:  permissionMsg,
				ToolName: toolCall.Function.Name,
			})
			// Note: The actual tool execution will be deferred until user responds
			continue
		case 2: // TrustSession - allow execution
			// Continue with execution
		default:
			// Unknown trust level, block for safety
			messages = append(messages, api.Message{
				Role:     "tool",
				Content:  fmt.Sprintf("⚠️  Tool '%s' execution blocked: Unknown trust level (%d). Please check Tools tab.", toolCall.Function.Name, trustLevel),
				ToolName: toolCall.Function.Name,
			})
			continue
		}

		// Execute the tool with the provided arguments using the unified tool system
		result, err := tooling.DefaultRegistry.ExecuteTool(toolCall.Function.Name, toolCall.Function.Arguments)
		if err != nil {
			// Create error message for tool execution failure
			messages = append(messages, api.Message{
				Role:     "tool",
				Content:  fmt.Sprintf("Error executing %s: %v", toolCall.Function.Name, err),
				ToolName: toolCall.Function.Name,
			})
			continue
		}

		// Format the result as JSON string for the tool response
		var resultStr string
		switch v := result.(type) {
		case string:
			resultStr = v
		default:
			// Convert result to JSON string for proper tool response format
			resultStr = fmt.Sprintf("%+v", result)
		}

		// Create tool response message
		messages = append(messages, api.Message{
			Role:     "tool",
			Content:  resultStr,
			ToolName: toolCall.Function.Name,
		})
	}

	return messages, nil
}

// calculateMessagesHeight calculates the total height of all messages
// with optimized performance
func (m Model) calculateMessagesHeight() int {
	// Cache message heights to avoid recomputing them repeatedly
	height := 0

	// Quick estimate if messages haven't changed
	if len(m.messages) > 0 {
		for _, msg := range m.messages {
			// Basic calculation: 1 line for header + content lines + 1 line for spacing
			contentWidth := m.width - 4 // Account for border
			wrappedLines := len(m.wrapText(msg.Content, contentWidth))
			height += 1 + wrappedLines + 1
		}
	}

	return height
}

// formatMessage formats a single message for display with performance optimization
func (m Model) formatMessage(msg Message) []string {
	// Preallocate slice with estimated capacity to avoid reallocations
	estimatedCapacity := 10 // header + some content lines + spacing
	lines := make([]string, 0, estimatedCapacity)

	// Header with role and timestamp
	timeStr := msg.Time.Format("15:04:05")

	// Use styles from the styles struct for better maintainability
	var header string
	if msg.Role == "user" {
		header = m.styles.userHeader.Render(fmt.Sprintf("User [%s]", timeStr))
	} else {
		header = m.styles.assistantHeader.Render(fmt.Sprintf("Assistant [%s]", timeStr))
	}
	lines = append(lines, header)

	// Message content (wrap to fit width)
	contentWidth := m.width - 4 // Account for border
	wrappedContent := m.wrapText(msg.Content, contentWidth)
	lines = append(lines, wrappedContent...)

	// Add spacing
	lines = append(lines, "")

	return lines
}
