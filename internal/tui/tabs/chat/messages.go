package chat

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ollama/ollama/api"
)

// sendMessage sends a message to Ollama using the Ollama API client
func (m Model) sendMessage(prompt string) tea.Cmd {
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

		// Create Ollama client with better error handling
		_, err := api.ClientFromEnvironment()
		if err != nil {
			return responseMsg{err: fmt.Errorf("failed to create Ollama client: %w", err)}
		}

		// Override the client's base URL with the configured Ollama URL
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
		if m.config.DefaultSystemPrompt != "" {
			messages = append(messages, api.Message{
				Role:    "system",
				Content: m.config.DefaultSystemPrompt,
			})
		}

		// Add all previous messages as context (preserving message history)
		for _, msg := range m.messages {
			messages = append(messages, api.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		// Add the current user message with RAG context if available
		messages = append(messages, api.Message{
			Role:    "user",
			Content: fullPrompt,
		})

		// Set options
		options := map[string]any{
			"temperature":    0.7,
			"repeat_last_n":  2,
			"repeat_penalty": 1.1,
		}

		// Create chat request with stream enabled (true is default, but we're explicit)
		stream := true
		chatRequest := &api.ChatRequest{
			Model:    m.config.ChatModel,
			Messages: messages,
			Stream:   &stream,
			Options:  options,
		}

		// Use ChatStream for real-time response with enhanced error handling
		var fullResponse strings.Builder
		var responseErr error

		err = client.Chat(m.ctx, chatRequest, func(response api.ChatResponse) error {
			// Check for context cancellation
			if m.ctx.Err() != nil {
				responseErr = m.ctx.Err()
				return responseErr
			}

			fullResponse.WriteString(response.Message.Content)
			return nil
		})

		if err != nil {
			if responseErr != nil {
				return responseMsg{err: fmt.Errorf("chat response error: %w", responseErr)}
			}
			return responseMsg{err: fmt.Errorf("chat request failed: %w", err)}
		}

		return responseMsg{content: fullResponse.String()}
	})
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
