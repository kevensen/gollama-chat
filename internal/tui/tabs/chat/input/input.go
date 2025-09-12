package input

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents an optimized input field for text entry
type Model struct {
	value     string         // Current input text
	cursor    int            // Cursor position
	width     int            // Available width for rendering
	height    int            // Height of input box
	style     lipgloss.Style // Style for the input box (unused but kept for compatibility)
	prompt    string         // Prompt prefix before the input
	loading   bool           // Whether the input is in loading state
	ragStatus string         // RAG status message to display during loading
}

// NewModel creates a new input model
func NewModel() Model {
	// Restore the border styling for better visual appearance
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8A7FD8")). // Use same purple color as messages border
		Height(3).
		Padding(0, 1) // Add horizontal padding for better readability

	return Model{
		value:     "",
		cursor:    0,
		width:     80, // Default width
		height:    3,
		style:     style,
		prompt:    "> ",
		loading:   false,
		ragStatus: "",
	}
}

// SetLoading sets the loading state
func (m *Model) SetLoading(loading bool) {
	m.loading = loading
	if !loading {
		m.ragStatus = "" // Clear RAG status when done loading
	}
}

// SetRAGStatus sets the RAG status message
func (m *Model) SetRAGStatus(status string) {
	m.ragStatus = status
}

// IsLoading returns whether the input is in loading state
func (m Model) IsLoading() bool {
	return m.loading
}

// SetSize updates the dimensions of the input component
func (m *Model) SetSize(width, height int) {
	// Update styling with new dimensions
	const fixedHeight = 3
	if m.width != width {
		m.width = width
		m.height = fixedHeight
		m.style = m.style.Width(width - 2).Height(fixedHeight) // Adjust for border
	}
}

// Value returns the current input text
func (m Model) Value() string {
	return m.value
}

// SetValue sets the input value and refreshes the display
func (m *Model) SetValue(value string) {
	m.value = value
	m.cursor = len(value)
}

// CursorPosition returns the current cursor position
func (m Model) CursorPosition() int {
	return m.cursor
}

// Clear resets the input value
func (m *Model) Clear() {
	m.value = ""
	m.cursor = 0
}

// Update handles events for the input component
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	// Skip processing if in loading state
	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Fast path for common character input - no need for full key handling
		if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
			// Direct character insertion for maximum responsiveness
			char := msg.String()
			if m.cursor == len(m.value) {
				m.value += char
			} else {
				// Use more efficient string building for mid-insertion
				result := make([]byte, 0, len(m.value)+1)
				result = append(result, m.value[:m.cursor]...)
				result = append(result, char...)
				result = append(result, m.value[m.cursor:]...)
				m.value = string(result)
			}
			m.cursor++
			return m, nil
		}
		return m.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, 3) // Keep height fixed
		return m, nil
	}
	return m, nil
}

// Backspace removes a character at the current cursor position
func (m *Model) Backspace() {
	if m.cursor > 0 {
		if m.cursor < len(m.value) {
			// Delete at cursor position
			m.value = m.value[:m.cursor-1] + m.value[m.cursor:]
		} else {
			// More efficient path for deleting at end
			m.value = m.value[:m.cursor-1]
		}
		m.cursor--
	}
}

// MoveCursorLeft moves the cursor one position to the left
func (m *Model) MoveCursorLeft() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// MoveCursorRight moves the cursor one position to the right
func (m *Model) MoveCursorRight() {
	if m.cursor < len(m.value) {
		m.cursor++
	}
}

// InsertCharacter inserts a character at the current cursor position
func (m *Model) InsertCharacter(char string) {
	if len(char) != 1 {
		return
	}

	// Fast path for appending at end (most common case)
	if m.cursor == len(m.value) {
		m.value += char
	} else {
		// Insert in the middle using efficient string manipulation
		var sb strings.Builder
		sb.Grow(len(m.value) + 1)
		sb.WriteString(m.value[:m.cursor])
		sb.WriteString(char)
		sb.WriteString(m.value[m.cursor:])
		m.value = sb.String()
	}
	m.cursor++
}

// handleKeyMsg processes keyboard input efficiently
func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "backspace":
		if m.cursor > 0 {
			if m.cursor < len(m.value) {
				// Use slice manipulation for better performance
				result := make([]byte, 0, len(m.value)-1)
				result = append(result, m.value[:m.cursor-1]...)
				result = append(result, m.value[m.cursor:]...)
				m.value = string(result)
			} else {
				// Simple truncation for end deletion
				m.value = m.value[:m.cursor-1]
			}
			m.cursor--
		}
		return m, nil

	case "left":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "right":
		if m.cursor < len(m.value) {
			m.cursor++
		}
		return m, nil

	case "home":
		m.cursor = 0
		return m, nil

	case "end":
		m.cursor = len(m.value)
		return m, nil

	case "ctrl+a":
		m.cursor = 0
		return m, nil

	case "ctrl+e":
		m.cursor = len(m.value)
		return m, nil
	}

	return m, nil
}

// View renders the input component with minimal overhead but nice styling
func (m *Model) View() string {
	// Build content efficiently using string builder for Unicode safety
	var content string

	if m.loading {
		// During loading, show thinking message and RAG status
		content = m.prompt + "Thinking..."
		if m.ragStatus != "" {
			content += " (" + m.ragStatus + ")"
		}
	} else {
		// Pre-calculate content length to avoid multiple string operations
		valueLen := len(m.value)

		if valueLen == 0 {
			// Simplified placeholder
			content = m.prompt + "Type your question...█"
		} else if m.cursor == valueLen {
			// Cursor at end - simple concatenation
			content = m.prompt + m.value + "█"
		} else {
			// Cursor in middle - use string builder for Unicode safety
			var sb strings.Builder
			sb.Grow(len(m.prompt) + valueLen + 3) // Pre-allocate capacity
			sb.WriteString(m.prompt)
			sb.WriteString(m.value[:m.cursor])
			sb.WriteString("█")
			sb.WriteString(m.value[m.cursor:])
			content = sb.String()
		}
	}

	// Apply styling efficiently
	return m.style.Render(content)
}
