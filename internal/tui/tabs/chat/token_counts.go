package chat

import (
	"strings"
	"unicode"
)

// updateTokenCount calculates the estimated token count for the current conversation
func (m *Model) updateTokenCount() {
	var totalText string

	// Include system prompt if configured
	if m.sessionSystemPrompt != "" {
		totalText += m.sessionSystemPrompt + " "
	}

	// Combine all message content
	for _, msg := range m.messages {
		totalText += msg.Content + " "
	}

	// Add current input if any
	if m.inputModel != nil && len(m.inputModel.Value()) > 0 {
		totalText += m.inputModel.Value()
	}

	// Estimate tokens using our own estimator
	m.tokenCount = estimateTokens(totalText)
}

// estimateTokens provides a rough estimate of GPT-style tokens
func estimateTokens(text string) int {
	// A very rough approximation: English text averages ~4 characters per token in GPT models
	const avgCharsPerToken = 4

	// Count characters excluding whitespace
	charCount := 0
	for _, char := range text {
		if !unicode.IsSpace(char) {
			charCount++
		}
	}

	// Add a small constant to account for spaces between words
	wordCount := len(strings.Fields(text))

	// Estimate token count
	tokenEstimate := (charCount + wordCount) / avgCharsPerToken
	if tokenEstimate < 1 && len(strings.TrimSpace(text)) > 0 {
		return 1
	}

	return tokenEstimate
}
