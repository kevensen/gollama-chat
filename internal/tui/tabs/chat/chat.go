package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/rag"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/chat/input"
)

// Message represents a chat message
type Message struct {
	Role    string    `json:"role"` // "user" or "assistant"
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// Model represents the chat tab model
type Model struct {
	config           *configuration.Config
	messages         []Message
	width            int
	height           int
	scrollOffset     int
	ragService       *rag.Service
	ctx              context.Context
	tokenCount       int  // Estimated token count for current conversation
	showSystemPrompt bool // Whether to show the system prompt

	// Optimized components
	inputModel   *input.Model
	messageCache *MessageCache
	styles       Styles

	// View caching
	cachedMessagesView      string
	cachedStatusView        string
	cachedSystemPromptView  string
	messagesNeedsUpdate     bool
	statusNeedsUpdate       bool
	systemPromptNeedsUpdate bool
}

// modelContextSizes maps model names to their approximate context window sizes
var modelContextSizes = map[string]int{
	"llama3.1":        8192,
	"llama3.1-8b":     8192,
	"llama3.1-70b":    8192,
	"llama3.2":        32768,
	"llama3.2-1b":     32768,
	"llama3.2-3b":     32768,
	"llama3.2-11b":    32768,
	"llama3.2-76b":    32768,
	"llama3.3":        128000,
	"llama3.3:latest": 128000,
	"llama3.3-8b":     128000,
	"llama3.3-70b":    128000,
	"llama3":          4096,
	"llama2":          4096,
	"mistral":         8192,
	"mistral-7b":      8192,
	"mistral-8x7b":    32768,
	"mixtral-8x7b":    32768,
	"codegemma":       32768,
	"gemma":           8192,
	"phi3":            4096,
	"neural-chat":     8192,
	"codellama":       16384,
	"llava":           4096,
	"vicuna":          4096,
	"orca-mini":       4096,
	"stable-lm":       4096,
	"mpt":             8192,
	"dolphin-phi":     4096,
}

// NewModel creates a new chat model
func NewModel(ctx context.Context, config *configuration.Config) Model {
	// Initialize RAG service
	ragService := rag.NewService(config)

	// Initialize input component
	inputModel := input.NewModel()

	// Create message cache
	messageCache := NewMessageCache()

	return Model{
		config:                  config,
		messages:                []Message{},
		ragService:              ragService,
		ctx:                     ctx,
		inputModel:              &inputModel,
		messageCache:            messageCache,
		styles:                  DefaultStyles(),
		messagesNeedsUpdate:     true,
		statusNeedsUpdate:       true,
		systemPromptNeedsUpdate: true,
		showSystemPrompt:        false, // Initially hidden
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
	// Initialize RAG service if it's enabled
	if m.config.RAGEnabled {
		return tea.Cmd(func() tea.Msg {
			err := m.ragService.Initialize(m.ctx)
			if err != nil {
				// Just log the error, don't prevent the app from starting
				return nil
			}
			return nil
		})
	}
	return nil
}

// Update handles messages and updates the chat model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	// Handle window size changes first
	if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
		// Store previous dimensions to detect actual changes
		prevWidth, prevHeight := m.width, m.height

		m.width = windowMsg.Width
		m.height = windowMsg.Height

		// Update the input model size with fixed height
		m.inputModel.SetSize(windowMsg.Width, 3)

		// Only indicate views need refreshing if dimensions actually changed
		if prevWidth != m.width || prevHeight != m.height {
			m.messagesNeedsUpdate = true
			m.statusNeedsUpdate = true

			// Invalidate message cache as we need to recalculate message wrapping
			m.messageCache.InvalidateCache()
		}

		return m, nil
	}

	// Handle key messages by first checking for chat-level controls, then delegating to input
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Don't process certain keys while loading
		if m.inputModel.IsLoading() && keyMsg.String() != "ctrl+c" && keyMsg.String() != "ctrl+l" {
			return m, nil
		}

		// Handle chat-level control keys first
		switch keyMsg.String() {
		case "enter":
			if strings.TrimSpace(m.inputModel.Value()) != "" {
				// Add user message
				userMsg := Message{
					Role:    "user",
					Content: m.inputModel.Value(),
					Time:    time.Now(),
				}
				m.messages = append(m.messages, userMsg)

				// Get the prompt and reset input
				prompt := m.inputModel.Value()
				m.inputModel.Clear()
				m.inputModel.SetLoading(true)

				// Mark messages for update
				m.messagesNeedsUpdate = true
				m.messageCache.InvalidateCache()

				// Update token count after adding user message
				m.updateTokenCount()
				m.statusNeedsUpdate = true

				return m, m.sendMessage(prompt)
			}

		case "ctrl+s":
			// Toggle system prompt display
			m.showSystemPrompt = !m.showSystemPrompt
			m.systemPromptNeedsUpdate = true
			m.messagesNeedsUpdate = true // Force layout refresh
			return m, nil

		case "ctrl+l":
			// Clear chat
			m.messages = []Message{}
			m.scrollOffset = 0
			m.messagesNeedsUpdate = true
			m.messageCache.InvalidateCache()
			return m, nil

		case "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
				m.messagesNeedsUpdate = true
			}
			return m, nil

		case "down":
			// Calculate max scroll
			messagesHeight := m.messageCache.GetTotalHeight(&m)

			// Get system prompt height
			systemPromptHeight := m.getSystemPromptHeight()
			availableHeight := m.height - 6 - systemPromptHeight // Reserve space for input area, status bar, and system prompt

			if messagesHeight > availableHeight {
				maxScroll := messagesHeight - availableHeight
				if m.scrollOffset < maxScroll {
					m.scrollOffset++
					m.messagesNeedsUpdate = true
				}
			}
			return m, nil

		case "pgup":
			// Page Up - scroll up by available height
			systemPromptHeight := m.getSystemPromptHeight()
			availableHeight := m.height - 6 - systemPromptHeight // Reserve space for input area, status bar, and system prompt
			pageSize := availableHeight - 1                      // Leave one line for context
			if pageSize < 1 {
				pageSize = 1
			}

			m.scrollOffset -= pageSize
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			m.messagesNeedsUpdate = true
			return m, nil

		case "pgdown":
			// Page Down - scroll down by available height
			systemPromptHeight := m.getSystemPromptHeight()
			availableHeight := m.height - 6 - systemPromptHeight // Reserve space for input area, status bar, and system prompt
			pageSize := max(
				// Leave one line for context
				availableHeight-1, 1)

			messagesHeight := m.messageCache.GetTotalHeight(&m)
			if messagesHeight > availableHeight {
				maxScroll := messagesHeight - availableHeight
				m.scrollOffset += pageSize
				if m.scrollOffset > maxScroll {
					m.scrollOffset = maxScroll
				}
				m.messagesNeedsUpdate = true
			}
			return m, nil

		default:
			// Delegate all other key handling (including text input) to the input component
			// Skip caching for maximum responsiveness
			updatedInputModel, cmd := m.inputModel.Update(msg)
			m.inputModel = &updatedInputModel
			// Don't cache input view for text input - let it render directly
			return m, cmd
		}
	}

	// Handle other message types
	switch msg := msg.(type) {

	case sendMessageMsg:
		return m, m.sendMessage(msg.message)

	case responseMsg:
		m.inputModel.SetLoading(false)
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

		// Mark messages for update but preserve layout dimensions
		m.messagesNeedsUpdate = true
		m.messageCache.InvalidateCache()

		// Update token count
		m.updateTokenCount()
		m.statusNeedsUpdate = true

		// Auto-scroll to bottom without changing dimensions
		m.scrollToBottom()
	}

	return m, nil
}

// View renders the chat tab
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Ensure consistent rendering regardless of scroll state or message count

	// Prepare the components only if they need updating
	var messagesView, statusView, systemPromptView string

	// System prompt view - only show if enabled
	var components []string
	if m.showSystemPrompt {
		if m.systemPromptNeedsUpdate || m.cachedSystemPromptView == "" {
			systemPromptView = m.renderSystemPrompt()
			m.cachedSystemPromptView = systemPromptView
			m.systemPromptNeedsUpdate = false
		} else {
			systemPromptView = m.cachedSystemPromptView
		}
		components = append(components, systemPromptView)
	}

	// Messages view - only recompute if needed
	if m.messagesNeedsUpdate || m.cachedMessagesView == "" {
		messagesView = m.messageCache.RenderAllMessages(&m)
		m.cachedMessagesView = messagesView
		m.messagesNeedsUpdate = false
	} else {
		messagesView = m.cachedMessagesView
	}
	components = append(components, messagesView)

	// Status bar - only recompute if needed
	if m.statusNeedsUpdate || m.cachedStatusView == "" {
		statusView = m.renderStatusBar()
		m.cachedStatusView = statusView
		m.statusNeedsUpdate = false
	} else {
		statusView = m.cachedStatusView
	}
	components = append(components, statusView)

	// Input view - render directly without caching for maximum responsiveness
	inputView := m.inputModel.View()
	components = append(components, inputView)

	// Join all components vertically
	return lipgloss.JoinVertical(
		lipgloss.Left,
		components...,
	)
}

// scrollToBottom scrolls to the bottom of the messages
func (m Model) scrollToBottom() {
	messagesHeight := m.calculateMessagesHeight()

	// Get system prompt height
	systemPromptHeight := m.getSystemPromptHeight()

	availableHeight := m.height - 6 - systemPromptHeight // Adjust for input area, status bar and system prompt
	if messagesHeight > availableHeight {
		m.scrollOffset = messagesHeight - availableHeight
	} else {
		m.scrollOffset = 0
	}
	// Don't trigger resize/reflow of the overall UI when scrolling
}

// wrapText wraps text to fit within the specified width
func (m Model) wrapText(text string, width int) []string {
	// Early return for edge cases
	if width <= 0 {
		return []string{text}
	}

	// Optimization for short texts that don't need wrapping
	if len(text) <= width {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	// Preallocate the result slice based on an estimate
	estimatedLines := (len(text) / width) + 1
	lines := make([]string, 0, estimatedLines)

	// Use strings.Builder for better performance
	var sb strings.Builder

	for _, word := range words {
		// Check if adding this word would exceed the width
		if sb.Len() == 0 {
			sb.WriteString(word)
		} else if sb.Len()+len(word)+1 <= width {
			sb.WriteString(" ")
			sb.WriteString(word)
		} else {
			// Line is full, append it and start a new one
			lines = append(lines, sb.String())
			sb.Reset()
			sb.WriteString(word)
		}
	}

	// Add the last line if there's anything left
	if sb.Len() > 0 {
		lines = append(lines, sb.String())
	}

	return lines
}

// renderStatusBar renders the status bar showing model and token information
func (m Model) renderStatusBar() string {
	// Use the pre-defined style from styles.go
	statusStyle := m.styles.statusBar.Width(m.width - 2)

	// Get model name
	modelInfo := fmt.Sprintf("Model: %s", m.config.ChatModel)

	// Get context window size
	contextSize := m.getModelContextSize(m.config.ChatModel)
	contextInfo := fmt.Sprintf("Context: %d", contextSize)

	// Get token information
	tokenInfo := fmt.Sprintf("Tokens: ~%d", m.tokenCount)

	// Calculate percentage of context used
	percentUsed := 0
	if contextSize > 0 {
		percentUsed = (m.tokenCount * 100) / contextSize
		if percentUsed > 100 {
			percentUsed = 100
		}
	}

	// Combine information with spacing
	status := fmt.Sprintf("%s | %s | %s (%d%%)", modelInfo, contextInfo, tokenInfo, percentUsed)

	return statusStyle.Render(status)
}
