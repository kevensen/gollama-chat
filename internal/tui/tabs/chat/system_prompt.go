package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// getSystemPromptHeight calculates the height of the system prompt panel
func (m Model) getSystemPromptHeight() int {
	if !m.showSystemPrompt {
		return 0
	}

	var systemPrompt string
	if m.systemPromptEditMode {
		systemPrompt = m.systemPromptEditor
	} else {
		systemPrompt = m.sessionSystemPrompt
	}

	// Estimate system prompt height based on content and borders
	lines := m.wrapText(systemPrompt, m.width-4) // Account for borders

	// Calculate the height but make sure it doesn't exceed a reasonable size
	// Add 4 for the borders, header, and padding, plus 1 for top margin
	promptHeight := len(lines) + 4 + 1

	// Set a minimum height when system prompt is visible to ensure it's substantial
	minHeight := (m.height * 2) / 5 // Minimum 2/5 of screen height (40%)
	if promptHeight < minHeight {
		promptHeight = minHeight
	}

	// Limit the system prompt height to a maximum of 1/2 of the screen height
	// to allow it to be about half the size of the chat pane
	maxHeight := m.height / 2
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

	var systemPrompt string
	var header string

	if m.systemPromptEditMode {
		// Edit mode - show editable content
		systemPrompt = m.systemPromptEditor
		header = "System Prompt - EDITING (ctrl+s to save, ctrl+r to restore default)"
	} else {
		// Display mode - show current session prompt
		systemPrompt = m.sessionSystemPrompt
		header = "System Prompt (ctrl+s to close, ctrl+e to edit)"
	}

	// Apply the style and wrap text to fit
	width := m.width - 4 // Account for borders and padding

	// Handle empty system prompt
	if strings.TrimSpace(systemPrompt) == "" {
		if m.systemPromptEditMode {
			systemPrompt = ""
		} else {
			systemPrompt = "No system prompt configured."
		}
	}

	// Wrap the text and limit its height
	wrappedLines := m.wrapText(systemPrompt, width)

	// Determine maximum content lines based on height constraints
	// (total height - header lines - padding)
	maxPromptHeight := m.getSystemPromptHeight() - 4
	if len(wrappedLines) > maxPromptHeight {
		// If text is too long, truncate and add an ellipsis
		wrappedLines = wrappedLines[:maxPromptHeight]
		if !m.systemPromptEditMode {
			wrappedLines[maxPromptHeight-1] += "..."
		}
	}
	wrappedText := strings.Join(wrappedLines, "\n")

	// Create a header and content
	content := fmt.Sprintf("%s\n\n%s", header, wrappedText)

	// Apply different styling based on edit mode and use calculated height
	calculatedHeight := m.getSystemPromptHeight()
	style := m.styles.systemPrompt.Width(m.width - 2).Height(calculatedHeight).MarginTop(1)
	if m.systemPromptEditMode {
		// Add a visual indicator for edit mode (different border color)
		style = style.BorderForeground(lipgloss.Color("#FFD700")) // Gold color for edit mode
	}

	return style.Render(content)
}
