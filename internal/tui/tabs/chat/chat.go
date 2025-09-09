package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

// Message represents a chat message
type Message struct {
	Role    string    `json:"role"` // "user" or "assistant"
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// OllamaRequest represents the request structure for Ollama API
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// OllamaResponse represents the response from Ollama API
type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Model represents the chat tab model
type Model struct {
	config       *configuration.Config
	messages     []Message
	input        string
	cursor       int
	loading      bool
	width        int
	height       int
	scrollOffset int
}

// NewModel creates a new chat model
func NewModel(config *configuration.Config) Model {
	return Model{
		config:   config,
		messages: []Message{},
		input:    "",
		cursor:   0,
		loading:  false,
	}
}

// sendMessageMsg is sent when a message should be sent to Ollama
type sendMessageMsg struct {
	message string
}

// responseMsg is sent when a response is received from Ollama
type responseMsg struct {
	content string
	err     error
}

// Init initializes the chat model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the chat model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.loading {
			// Don't process input while loading
			return m, nil
		}

		switch msg.String() {
		case "enter":
			if strings.TrimSpace(m.input) != "" {
				// Add user message
				userMsg := Message{
					Role:    "user",
					Content: m.input,
					Time:    time.Now(),
				}
				m.messages = append(m.messages, userMsg)

				// Send message to Ollama
				prompt := m.input
				m.input = ""
				m.cursor = 0
				m.loading = true

				return m, m.sendMessage(prompt)
			}

		case "ctrl+l":
			// Clear chat
			m.messages = []Message{}
			m.scrollOffset = 0

		case "backspace":
			if m.cursor > 0 {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
			}

		case "left":
			if m.cursor > 0 {
				m.cursor--
			}

		case "right":
			if m.cursor < len(m.input) {
				m.cursor++
			}

		case "home":
			m.cursor = 0

		case "end":
			m.cursor = len(m.input)

		case "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}

		case "down":
			// Calculate max scroll
			messagesHeight := m.calculateMessagesHeight()
			availableHeight := m.height - 6 // Reserve space for input area
			if messagesHeight > availableHeight {
				maxScroll := messagesHeight - availableHeight
				if m.scrollOffset < maxScroll {
					m.scrollOffset++
				}
			}

		default:
			// Add character to input
			if len(msg.String()) == 1 {
				m.input = m.input[:m.cursor] + msg.String() + m.input[m.cursor:]
				m.cursor++
			}
		}

	case sendMessageMsg:
		return m, m.sendMessage(msg.message)

	case responseMsg:
		m.loading = false
		if msg.err != nil {
			// Add error message
			errorMsg := Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Error: %s", msg.err.Error()),
				Time:    time.Now(),
			}
			m.messages = append(m.messages, errorMsg)
		} else {
			// Add assistant response
			assistantMsg := Message{
				Role:    "assistant",
				Content: msg.content,
				Time:    time.Now(),
			}
			m.messages = append(m.messages, assistantMsg)
		}

		// Auto-scroll to bottom
		m.scrollToBottom()
	}

	return m, nil
}

// View renders the chat tab
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Messages area
	messagesView := m.renderMessages()

	// Input area
	inputView := m.renderInput()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		messagesView,
		inputView,
	)
}

// renderMessages renders the chat messages
func (m Model) renderMessages() string {
	if len(m.messages) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Align(lipgloss.Center).
			Width(m.width - 2).
			Height(m.height - 6) // Reserve more space for input area and padding

		return emptyStyle.Render("No messages yet. Type a message and press Enter to start chatting!")
	}

	var lines []string

	for _, msg := range m.messages {
		lines = append(lines, m.formatMessage(msg)...)
	}

	// Apply scroll offset
	if m.scrollOffset > 0 && m.scrollOffset < len(lines) {
		lines = lines[m.scrollOffset:]
	}

	// Limit to available height
	availableHeight := m.height - 6 // Reserve more space for input area
	if len(lines) > availableHeight {
		lines = lines[len(lines)-availableHeight:]
	}

	messagesStyle := lipgloss.NewStyle().
		Width(m.width - 2).
		Height(m.height - 6). // Match the availableHeight calculation
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	content := strings.Join(lines, "\n")
	return messagesStyle.Render(content)
}

// formatMessage formats a single message for display
func (m Model) formatMessage(msg Message) []string {
	var lines []string

	// Header with role and timestamp
	timeStr := msg.Time.Format("15:04:05")
	var headerStyle lipgloss.Style

	if msg.Role == "user" {
		headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)
	} else {
		headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)
	}

	header := headerStyle.Render(fmt.Sprintf("%s [%s]", strings.Title(msg.Role), timeStr))
	lines = append(lines, header)

	// Message content (wrap to fit width)
	contentWidth := m.width - 4 // Account for border
	wrappedContent := m.wrapText(msg.Content, contentWidth)
	lines = append(lines, wrappedContent...)

	// Add spacing
	lines = append(lines, "")

	return lines
}

// renderInput renders the input area
func (m Model) renderInput() string {
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Width(m.width - 2). // Reduce width to prevent wrapping
		Height(3)

	prompt := "> "

	// Create input display with cursor
	input := m.input
	if m.cursor <= len(input) {
		if m.cursor == len(input) {
			input += "█" // Cursor at end
		} else {
			input = input[:m.cursor] + "█" + input[m.cursor+1:]
		}
	}

	status := ""
	if m.loading {
		status = " [Thinking...]"
	}

	content := prompt + input + status

	return inputStyle.Render(content)
}

// sendMessage sends a message to Ollama
func (m Model) sendMessage(prompt string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		req := OllamaRequest{
			Model:  m.config.ChatModel,
			Prompt: prompt,
			Stream: false,
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			return responseMsg{err: err}
		}

		resp, err := http.Post(
			m.config.OllamaURL+"/api/generate",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			return responseMsg{err: err}
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return responseMsg{err: err}
		}

		var ollamaResp OllamaResponse
		if err := json.Unmarshal(body, &ollamaResp); err != nil {
			return responseMsg{err: err}
		}

		return responseMsg{content: ollamaResp.Response}
	})
}

// calculateMessagesHeight calculates the total height of all messages
func (m Model) calculateMessagesHeight() int {
	height := 0
	for _, msg := range m.messages {
		height += len(m.formatMessage(msg))
	}
	return height
}

// scrollToBottom scrolls to the bottom of the messages
func (m Model) scrollToBottom() {
	messagesHeight := m.calculateMessagesHeight()
	availableHeight := m.height - 6 // Match other height calculations
	if messagesHeight > availableHeight {
		m.scrollOffset = messagesHeight - availableHeight
	}
}

// wrapText wraps text to fit within the specified width
func (m Model) wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	currentLine := ""

	for _, word := range words {
		if len(currentLine) == 0 {
			currentLine = word
		} else if len(currentLine)+len(word)+1 <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}

	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}

	return lines
}
