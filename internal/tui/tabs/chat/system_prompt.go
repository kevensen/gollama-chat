package chat

import (
	"fmt"
	"strings"
)

// getSystemPromptHeight calculates the height of the system prompt panel
func (m Model) getSystemPromptHeight() int {
	if !m.showSystemPrompt {
		return 0
	}

	// Estimate system prompt height based on content and borders
	systemPrompt := m.config.DefaultSystemPrompt
	lines := m.wrapText(systemPrompt, m.width-4) // Account for borders

	// Calculate the height but make sure it doesn't exceed a reasonable size
	// Add 4 for the borders, header, and padding, plus 1 for top margin
	promptHeight := len(lines) + 4 + 1

	// Limit the system prompt height to a maximum of 1/3 of the screen height
	// to ensure it doesn't take too much space
	maxHeight := m.height / 3
	if promptHeight > maxHeight {
		promptHeight = maxHeight
	}

	return promptHeight
}

// renderSystemPrompt renders the system prompt panel
func (m Model) renderSystemPrompt() string {
	if !m.showSystemPrompt {
		return ""
	}

	// Get the system prompt from config
	systemPrompt := m.config.DefaultSystemPrompt

	// Apply the style and wrap text to fit
	width := m.width - 4 // Account for borders and padding

	// Handle empty system prompt
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = "No system prompt configured."
	}

	// Wrap the text and limit its height
	wrappedLines := m.wrapText(systemPrompt, width)

	// Determine maximum content lines based on height constraints
	// (total height - header lines - padding)
	maxPromptHeight := m.getSystemPromptHeight() - 4
	if len(wrappedLines) > maxPromptHeight {
		// If text is too long, truncate and add an ellipsis
		wrappedLines = wrappedLines[:maxPromptHeight]
		wrappedLines[maxPromptHeight-1] += "..."
	}
	wrappedText := strings.Join(wrappedLines, "\n")

	// Create a header and content
	header := "System Prompt (ctrl+s to toggle)"
	content := fmt.Sprintf("%s\n\n%s", header, wrappedText)

	// Apply styling without constraining height to avoid cutting off borders
	// Add a small top margin to ensure the top border is visible
	return m.styles.systemPrompt.
		Width(m.width - 2).
		MarginTop(1). // Add top margin to push down from tab bar
		Render(content)
}
