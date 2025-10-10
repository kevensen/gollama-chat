package chat

import (
	"github.com/charmbracelet/lipgloss"
)

// Styles holds precomputed styles for the chat UI
type Styles struct {
	// Style for user message headers
	userHeader lipgloss.Style

	// Style for assistant message headers
	assistantHeader lipgloss.Style

	// Style for the message container
	messages lipgloss.Style

	// Style for the empty message state
	emptyMessages lipgloss.Style

	// Style for the status bar
	statusBar lipgloss.Style

	// Style for the system prompt panel
	systemPrompt lipgloss.Style

	// Style for bold text within message content
	boldText lipgloss.Style
}

// DefaultStyles creates default styles for the chat UI
func DefaultStyles() Styles {
	return Styles{
		userHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true),

		assistantHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true),

		messages: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8A7FD8")).
			Padding(0, 1), // Add horizontal padding for better readability

		emptyMessages: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Align(lipgloss.Center),

		statusBar: lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color("240")).
			PaddingLeft(1),

		systemPrompt: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("13")). // Purple border for distinction
			Padding(1, 1).
			Foreground(lipgloss.Color("15")), // Normal white text, no background

		boldText: lipgloss.NewStyle().
			Bold(true),
	}
}
