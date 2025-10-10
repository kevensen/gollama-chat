package chat

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/oklog/ulid/v2"
	"github.com/ollama/ollama/api"

	"github.com/kevensen/gollama-chat/internal/agents"
	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
	"github.com/kevensen/gollama-chat/internal/rag"
	"github.com/kevensen/gollama-chat/internal/tooling"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/chat/input"
)

// Message represents a chat message
type Message struct {
	Role      string         `json:"role"` // "user", "assistant", or "tool"
	Content   string         `json:"content"`
	Time      time.Time      `json:"time"`
	ULID      string         `json:"ulid"`                 // ULID for traceability
	ToolName  string         `json:"tool_name,omitempty"`  // For tool messages
	Hidden    bool           `json:"hidden,omitempty"`     // Whether to hide from TUI display
	ToolCalls []ToolCallInfo `json:"tool_calls,omitempty"` // For assistant messages with tool calls
}

// ToolCallInfo stores tool call information for persistence
type ToolCallInfo struct {
	FunctionName string         `json:"function_name"`
	Arguments    map[string]any `json:"arguments"`
}

// ToolPermissionRequest represents a pending tool permission request
type ToolPermissionRequest struct {
	ToolCall    api.ToolCall
	ToolName    string
	Description string
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

	// AGENTS.md integration
	agentsFile *agents.AgentsFile // Detected AGENTS.md file for project context

	// Session system prompt feature
	sessionSystemPrompt       string // Current session system prompt (not persisted)
	sessionSystemPromptManual bool   // Whether session prompt was manually modified (takes precedence over default)
	systemPromptEditMode      bool   // Whether we're in edit mode for the system prompt
	systemPromptEditor        string // Content being edited in the system prompt editor

	// Performance optimization: Cache model context size
	cachedModelName   string // Track which model's context size we cached
	cachedContextSize int    // Cached context size to avoid API calls during rendering

	// Optimized components
	inputModel   *input.Model
	messageCache *MessageCache
	styles       Styles

	// Tool permission prompt state
	pendingToolPermission *ToolPermissionRequest
	waitingForPermission  bool
	pendingToolCalls      map[string]api.ToolCall // Store complete tool calls by tool name

	// ULID for conversation traceability
	currentConversationULID string // The ULID for the current user prompt and its entire flow

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

// ULID generator for message traceability
var ulidGenerator = ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)

// debugLog writes debug messages to a file for troubleshooting
func debugLog(message string) {
	f, err := os.OpenFile("/tmp/gollama-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("15:04:05.000")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message))
}

// generateULID creates a new ULID for message traceability
func generateULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulidGenerator).String()
}

// logConversationEvent logs user prompts and assistant responses with ULID
func logConversationEvent(messageID, role, content, model string) {
	logger := logging.WithComponent("conversation")
	logger.Info("Conversation message",
		"message_id", messageID,
		"role", role,
		"content_length", len(content),
		"content_preview", contentPreview(content, 100),
		"model", model,
		"timestamp", time.Now().Format(time.RFC3339),
	)
}

// contentPreview returns a truncated preview of content for logging
func contentPreview(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "..."
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
		config:                    config,
		messages:                  []Message{},
		ragService:                ragService,
		ctx:                       ctx,
		inputModel:                &inputModel,
		messageCache:              messageCache,
		styles:                    DefaultStyles(),
		messagesNeedsUpdate:       true,
		statusNeedsUpdate:         true,
		systemPromptNeedsUpdate:   true,
		showSystemPrompt:          false,                         // Initially hidden
		pendingToolCalls:          make(map[string]api.ToolCall), // Initialize tool calls map
		sessionSystemPrompt:       config.DefaultSystemPrompt,    // Initialize with default
		sessionSystemPromptManual: false,                         // Initially uses default prompt
		systemPromptEditMode:      false,
		systemPromptEditor:        "",
	}
}

// NewModelWithAgents creates a new chat model with AGENTS.md integration
func NewModelWithAgents(ctx context.Context, config *configuration.Config, agentsFile *agents.AgentsFile) Model {
	// Initialize RAG service
	ragService := rag.NewService(config)

	// Initialize input component
	inputModel := input.NewModel()

	// Create message cache
	messageCache := NewMessageCache()

	// Build system prompt with agents file content if available
	systemPrompt := config.DefaultSystemPrompt
	if agentsFile != nil {
		logger := logging.WithComponent("chat")
		logger.Info("Initializing chat model with AGENTS.md content",
			"agents_file_path", agentsFile.Path,
			"agents_content_length", len(agentsFile.Content),
			"base_prompt_length", len(config.DefaultSystemPrompt))
		systemPrompt += agentsFile.FormatAsSystemPromptAddition()
		logger.Info("AGENTS.md content added to system prompt",
			"final_prompt_length", len(systemPrompt))
	} else {
		logger := logging.WithComponent("chat")
		logger.Debug("Initializing chat model without AGENTS.md content")
	}

	return Model{
		config:                    config,
		messages:                  []Message{},
		ragService:                ragService,
		ctx:                       ctx,
		agentsFile:                agentsFile, // Store the agents file for reference
		inputModel:                &inputModel,
		messageCache:              messageCache,
		styles:                    DefaultStyles(),
		messagesNeedsUpdate:       true,
		statusNeedsUpdate:         true,
		systemPromptNeedsUpdate:   true,
		showSystemPrompt:          false,                         // Initially hidden
		pendingToolCalls:          make(map[string]api.ToolCall), // Initialize tool calls map
		sessionSystemPrompt:       systemPrompt,                  // Initialize with agents-enhanced prompt
		sessionSystemPromptManual: false,                         // Initially uses default prompt (enhanced with agents)
		systemPromptEditMode:      false,
		systemPromptEditor:        "",
	}
}

// sendMessageMsg is sent when a message should be sent to Ollama
type sendMessageMsg struct {
	message          string
	conversationULID string // ULID for the entire conversation flow
}

// responseMsg is sent when a response is received from Ollama
type responseMsg struct {
	content            string
	err                error
	additionalMessages []Message // For tool calls and results that need to be added to history
	conversationULID   string    // ULID for the entire conversation flow
}

// ragStatusMsg is sent to update RAG status in the input
type ragStatusMsg struct {
	status string
}

// Init initializes the chat model
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Initialize RAG service if it's enabled
	if m.config.RAGEnabled {
		ragCmd := tea.Cmd(func() tea.Msg {
			err := m.ragService.Initialize(m.ctx)
			if err != nil {
				// Just log the error, don't prevent the app from starting
				return nil
			}
			return nil
		})
		cmds = append(cmds, ragCmd)
	}

	// Pre-fetch model context size in background to avoid UI blocking
	contextCmd := tea.Cmd(func() tea.Msg {
		// This runs in background and updates the cache
		if m.config != nil && m.config.ChatModel != "" {
			// Force a context size fetch to populate cache
			_ = m.getModelContextSize(m.config.ChatModel)
		}
		return nil
	})
	cmds = append(cmds, contextCmd)

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
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

		// Fast path for text input - delegate immediately to input component for maximum responsiveness
		key := keyMsg.String()
		if len(key) == 1 && key >= " " && key <= "~" {
			// DEBUG: Log character input and current state
			debugLog(fmt.Sprintf("Character '%s', systemPromptEditMode=%t, showSystemPrompt=%t", key, m.systemPromptEditMode, m.showSystemPrompt))
			if m.systemPromptEditMode {
				// Handle text input for system prompt editing
				debugLog(fmt.Sprintf("Adding '%s' to system prompt editor", key))
				m.systemPromptEditor += key
				m.systemPromptNeedsUpdate = true
				return m, nil
			} else {
				debugLog(fmt.Sprintf("Delegating '%s' to input model", key))
				updatedInputModel, cmd := m.inputModel.Update(msg)
				m.inputModel = &updatedInputModel
				return m, cmd
			}
		}

		// Handle chat-level control keys
		switch key {
		case "enter":
			if m.systemPromptEditMode {
				// Add newline to system prompt editor
				m.systemPromptEditor += "\n"
				m.systemPromptNeedsUpdate = true
				return m, nil
			} else if m.waitingForPermission && m.pendingToolPermission != nil {
				// Handle permission response
				response := strings.TrimSpace(strings.ToLower(m.inputModel.Value()))
				m.inputModel.Clear()

				switch response {
				case "y", "yes":
					// Execute the tool with current trust level
					return m.executeApprovedTool(false)
				case "n", "no":
					// Deny tool execution
					return m.denyToolExecution()
				case "t", "trust":
					// Execute the tool and update trust level to Session
					return m.executeApprovedTool(true)
				default:
					// Invalid response, show message and keep waiting
					invalidULID := generateULID()
					invalidMsg := Message{
						Role:    "system",
						Content: "Please respond with 'y' (yes), 'n' (no), or 't' (trust for session)",
						Time:    time.Now(),
						ULID:    invalidULID,
					}
					m.messages = append(m.messages, invalidMsg)
					m.messagesNeedsUpdate = true
					m.messageCache.InvalidateCache()

					// Log system message
					logConversationEvent(invalidULID, "system", invalidMsg.Content, m.config.ChatModel)
					return m, nil
				}
			} else if strings.TrimSpace(m.inputModel.Value()) != "" {
				userInput := strings.TrimSpace(m.inputModel.Value())

				// Handle /clear command
				if userInput == "/clear" {
					// Clear chat history
					m.messages = []Message{}
					m.inputModel.Clear()
					m.messagesNeedsUpdate = true
					m.messageCache.InvalidateCache()

					// Update token count after clearing
					m.updateTokenCount()
					m.statusNeedsUpdate = true

					// Log the clear action
					logger := logging.WithComponent("chat")
					logger.Info("Chat history cleared by user command")

					return m, nil
				}

				// Generate a single ULID for the entire conversation flow
				conversationULID := generateULID()
				m.currentConversationULID = conversationULID

				// Add user message
				userMsg := Message{
					Role:    "user",
					Content: userInput,
					Time:    time.Now(),
					ULID:    conversationULID, // Use conversation ULID for user message
				}
				m.messages = append(m.messages, userMsg)

				// Log user prompt with conversation ULID
				logConversationEvent(conversationULID, "user", userMsg.Content, m.config.ChatModel)

				// Get the prompt and reset input
				prompt := userInput
				m.inputModel.Clear()
				m.inputModel.SetLoading(true)

				// Set initial RAG status if RAG is enabled
				if m.config.RAGEnabled {
					if m.ragService != nil && m.ragService.IsReady() {
						m.inputModel.SetRAGStatus("Searching documents...")
					} else {
						m.inputModel.SetRAGStatus("RAG not ready")
					}
				}

				// Mark messages for update
				m.messagesNeedsUpdate = true
				m.messageCache.InvalidateCache()

				// Update token count after adding user message
				m.updateTokenCount()
				m.statusNeedsUpdate = true

				return m, m.sendMessage(prompt, conversationULID)
			}

		case "ctrl+s":
			// Handle system prompt toggle and save
			debugLog(fmt.Sprintf("Ctrl+S pressed, current state: showSystemPrompt=%t, systemPromptEditMode=%t", m.showSystemPrompt, m.systemPromptEditMode))
			if m.showSystemPrompt && m.systemPromptEditMode {
				// Save the edited prompt and exit edit mode
				debugLog("Saving prompt and exiting edit mode")
				m.sessionSystemPrompt = m.systemPromptEditor

				// If the saved prompt matches the default, reset manual flag to allow future updates
				if m.sessionSystemPrompt == m.config.DefaultSystemPrompt {
					m.sessionSystemPromptManual = false
					debugLog("Saved prompt matches default, resetting manual flag")
				} else {
					m.sessionSystemPromptManual = true // Mark as manually modified
					debugLog("Saved prompt differs from default, setting manual flag")
				}

				m.systemPromptEditMode = false
				m.systemPromptEditor = ""
				m.systemPromptNeedsUpdate = true
				return m, nil
			} else if m.showSystemPrompt {
				// Close system prompt pane
				debugLog("Closing system prompt pane")
				m.showSystemPrompt = false
				m.systemPromptNeedsUpdate = true
				m.messagesNeedsUpdate = true // Force layout refresh
				return m, nil
			} else {
				// Open system prompt pane
				debugLog("Opening system prompt pane")
				m.showSystemPrompt = true
				m.systemPromptNeedsUpdate = true
				m.messagesNeedsUpdate = true // Force layout refresh
				return m, nil
			}

		case "ctrl+e":
			// Only handle system prompt editing if system prompt is visible and has focus
			// Otherwise, delegate to input model for cursor movement to end
			debugLog(fmt.Sprintf("Ctrl+E pressed, current state: showSystemPrompt=%t, systemPromptEditMode=%t", m.showSystemPrompt, m.systemPromptEditMode))
			if m.showSystemPrompt && !m.systemPromptEditMode {
				// System prompt is visible but not in edit mode - enter edit mode
				m.systemPromptEditMode = true
				m.systemPromptEditor = "" // Clear the prompt as requested
				m.systemPromptNeedsUpdate = true
				debugLog("Set systemPromptEditMode=true, cleared editor")
				return m, nil
			} else if !m.showSystemPrompt || m.systemPromptEditMode {
				// System prompt not visible OR already in edit mode - delegate to input model for cursor movement
				debugLog("Delegating ctrl+e to input model for cursor movement")
				updatedInputModel, cmd := m.inputModel.Update(msg)
				m.inputModel = &updatedInputModel
				return m, cmd
			}
			return m, nil

		case "ctrl+r":
			// Restore default system prompt when in edit mode
			if m.showSystemPrompt && m.systemPromptEditMode {
				m.systemPromptEditor = m.config.DefaultSystemPrompt
				m.systemPromptNeedsUpdate = true
				debugLog("Restored default system prompt in editor")
				return m, nil
			}

		case "ctrl+shift+c":
			// Copy conversation history to clipboard
			err := m.copyConversationToClipboard()
			if err != nil {
				// Add error message to show copy failed
				errorULID := generateULID()
				errorMsg := Message{
					Role:    "system",
					Content: fmt.Sprintf("Failed to copy conversation to clipboard: %s", err.Error()),
					Time:    time.Now(),
					ULID:    errorULID,
				}
				m.messages = append(m.messages, errorMsg)
				m.messagesNeedsUpdate = true
				m.messageCache.InvalidateCache()

				// Log error
				logConversationEvent(errorULID, "system", errorMsg.Content, m.config.ChatModel)
			} else {
				// Add success message to show copy succeeded
				successULID := generateULID()
				successMsg := Message{
					Role:    "system",
					Content: "Conversation history copied to clipboard.",
					Time:    time.Now(),
					ULID:    successULID,
				}
				m.messages = append(m.messages, successMsg)
				m.messagesNeedsUpdate = true
				m.messageCache.InvalidateCache()

				// Log success
				logConversationEvent(successULID, "system", successMsg.Content, m.config.ChatModel)
			}
			return m, nil

		case "ctrl+l":
			// Clear chat
			m.messages = []Message{}
			m.scrollOffset = 0
			m.messagesNeedsUpdate = true
			m.messageCache.InvalidateCache()
			return m, nil

		case "backspace", "left", "right", "home", "end", "ctrl+a":
			// Delegate cursor and deletion operations directly to input
			if m.systemPromptEditMode {
				// Handle system prompt editing keys
				switch key {
				case "backspace":
					if len(m.systemPromptEditor) > 0 {
						m.systemPromptEditor = m.systemPromptEditor[:len(m.systemPromptEditor)-1]
						m.systemPromptNeedsUpdate = true
					}
				}
				return m, nil
			} else {
				updatedInputModel, cmd := m.inputModel.Update(msg)
				m.inputModel = &updatedInputModel
				return m, cmd
			}

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
			// For all other keys, delegate to input component for maximum responsiveness
			// This includes character input, special key combinations, etc.
			debugLog(fmt.Sprintf("Default case: key='%s', systemPromptEditMode=%t", key, m.systemPromptEditMode))
			if m.systemPromptEditMode {
				// Handle text input for system prompt editing
				if len(key) == 1 && key >= " " && key <= "~" {
					debugLog(fmt.Sprintf("Default case: Adding '%s' to system prompt editor", key))
					m.systemPromptEditor += key
					m.systemPromptNeedsUpdate = true
				} else {
					debugLog(fmt.Sprintf("Default case: Non-printable key '%s' ignored in edit mode", key))
				}
				return m, nil
			} else {
				debugLog(fmt.Sprintf("Default case: Delegating '%s' to input model", key))
				updatedInputModel, cmd := m.inputModel.Update(msg)
				m.inputModel = &updatedInputModel
				return m, cmd
			}
		}
	}

	// Handle other message types
	switch msg := msg.(type) {

	case sendMessageMsg:
		return m, m.sendMessage(msg.message, msg.conversationULID)

	case ragStatusMsg:
		// Update the input's RAG status
		m.inputModel.SetRAGStatus(msg.status)
		return m, nil

	case responseMsg:
		m.inputModel.SetLoading(false)
		if msg.err != nil {
			// Add error message using conversation ULID for traceability
			errorMsg := Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Error: %s", msg.err.Error()),
				Time:    time.Now(),
				ULID:    msg.conversationULID, // Use conversation ULID for traceability
			}
			m.messages = append(m.messages, errorMsg)

			// Log error message with conversation ULID
			logConversationEvent(msg.conversationULID, "assistant", errorMsg.Content, m.config.ChatModel)
		} else {
			// Check for tool permission requests in additional messages
			permissionFound := false
			for _, additionalMsg := range msg.additionalMessages {
				// Check if this is a permission request message
				if additionalMsg.Role == "tool" && strings.HasPrefix(additionalMsg.Content, "❓ Tool '") &&
					strings.Contains(additionalMsg.Content, "Allow execution? (y)es / (n)o / (t)rust for session") {

					// Extract tool name and arguments from the message
					toolName := additionalMsg.ToolName
					if toolName != "" {
						// Extract arguments from the TOOL_CALL_DATA section
						var arguments map[string]any
						if strings.Contains(additionalMsg.Content, "TOOL_CALL_DATA:") {
							// Parse the tool call data - this is a simplified approach
							// The format is: TOOL_CALL_DATA:toolname:arguments
							parts := strings.Split(additionalMsg.Content, "TOOL_CALL_DATA:")
							if len(parts) > 1 {
								// For now, we'll reconstruct the arguments based on the tool
								// This is not ideal but works for the filesystem tool
								if toolName == "filesystem_read" && strings.Contains(additionalMsg.Content, "get_working_directory") {
									arguments = map[string]any{
										"action": "get_working_directory",
									}
								} else {
									arguments = make(map[string]any)
								}
							} else {
								arguments = make(map[string]any)
							}
						} else {
							arguments = make(map[string]any)
						}

						// Create the complete tool call
						toolCall := api.ToolCall{
							Function: api.ToolCallFunction{
								Name:      toolName,
								Arguments: arguments,
							},
						}

						// Store the complete tool call for later execution
						m.pendingToolCalls[toolName] = toolCall

						// Set up permission request with the actual tool call
						m.pendingToolPermission = &ToolPermissionRequest{
							ToolCall:    toolCall,
							ToolName:    toolName,
							Description: fmt.Sprintf("Tool '%s' requires permission to execute", toolName),
						}
						m.waitingForPermission = true
						m.inputModel.SetPlaceholder("Type your response...")
						permissionFound = true

						// Add a cleaned up version of the permission request message to display
						cleanContent := strings.Split(additionalMsg.Content, "\n\nTOOL_CALL_DATA:")[0]
						cleanMsg := Message{
							Role:     additionalMsg.Role,
							Content:  cleanContent,
							Time:     time.Now(),
							ULID:     msg.conversationULID, // Use conversation ULID for traceability
							ToolName: additionalMsg.ToolName,
						}
						m.messages = append(m.messages, cleanMsg)

						// Log tool permission request with conversation ULID
						logConversationEvent(msg.conversationULID, additionalMsg.Role, cleanContent, m.config.ChatModel)
					}
				} else {
					// Skip empty messages
					if strings.TrimSpace(additionalMsg.Content) != "" {
						// Use conversation ULID if additional message doesn't have one
						if additionalMsg.ULID == "" {
							additionalMsg.ULID = msg.conversationULID
						}
						m.messages = append(m.messages, additionalMsg)

						// Log additional message (usually tool responses) with conversation ULID
						logConversationEvent(additionalMsg.ULID, additionalMsg.Role, additionalMsg.Content, m.config.ChatModel)
					}
				}
			}

			// If we found a permission request, don't add the final assistant response yet
			if !permissionFound {
				// Always add assistant response to maintain conversation flow
				var responseContent string
				if strings.TrimSpace(msg.content) == "" {
					// If LLM returned empty/whitespace content, provide helpful message
					responseContent = "I'm unable to provide a response to that question. This might be because I don't have access to the necessary tools or information to answer it properly."
				} else {
					responseContent = msg.content
				}

				assistantMsg := Message{
					Role:    "assistant",
					Content: responseContent,
					Time:    time.Now(),
					ULID:    msg.conversationULID, // Use conversation ULID for traceability
				}
				m.messages = append(m.messages, assistantMsg)

				// Log assistant response with conversation ULID (log original content for debugging)
				logContent := responseContent
				if strings.TrimSpace(msg.content) == "" {
					logContent = fmt.Sprintf("[Empty LLM response] %s", responseContent)
				}
				logConversationEvent(msg.conversationULID, "assistant", logContent, m.config.ChatModel)
			}
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
	// Use a simpler rendering approach to minimize latency
	inputView := m.inputModel.View()
	components = append(components, inputView)

	// Join all components vertically with minimal processing
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

// wrapText wraps text to fit within the specified width while preserving formatting
func (m Model) wrapText(text string, width int) []string {
	// Early return for edge cases
	if width <= 0 {
		return []string{text}
	}

	return m.formatTextWithMarkdown(text, width)
}

// formatTextWithMarkdown processes text with enhanced formatting support
func (m Model) formatTextWithMarkdown(text string, width int) []string {
	var result []string

	// Split by newlines first to preserve explicit line breaks
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		// Handle different types of content
		if strings.TrimSpace(line) == "" {
			// Preserve empty lines
			result = append(result, "")
		} else if m.isCodeBlock(line) {
			// Handle code blocks (preserve indentation and tabs)
			result = append(result, m.formatCodeLine(line, width))
		} else if m.isBulletPoint(line) {
			// Handle bullet points
			result = append(result, m.formatBulletLine(line, width)...)
		} else if m.isNumberedList(line) {
			// Handle numbered lists
			result = append(result, m.formatNumberedLine(line, width)...)
		} else {
			// Regular text with word wrapping
			result = append(result, m.wrapRegularText(line, width)...)
		}
	}

	return result
}

// isCodeBlock checks if a line appears to be code (starts with spaces/tabs or has code-like patterns)
func (m Model) isCodeBlock(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	// Check for indentation (4+ spaces or tabs)
	if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
		return true
	}

	// Check for code block markers
	if strings.HasPrefix(trimmed, "```") {
		return true
	}

	// Check for common code patterns
	codePatterns := []string{
		"func ", "def ", "class ", "import ", "from ", "package ",
		"const ", "var ", "let ", "if (", "for (", "while (",
		"    }", "    {", "};", "){", "(){",
	}

	for _, pattern := range codePatterns {
		if strings.Contains(trimmed, pattern) {
			return true
		}
	}

	return false
}

// formatCodeLine handles code lines by preserving indentation and tabs
func (m Model) formatCodeLine(line string, width int) string {
	// Convert tabs to spaces for consistent display (4 spaces per tab)
	expanded := strings.ReplaceAll(line, "\t", "    ")

	// If the line is too long, truncate with ellipsis rather than wrap
	if len(expanded) > width-3 {
		return expanded[:width-3] + "..."
	}

	return expanded
}

// isBulletPoint checks if a line is a bullet point
func (m Model) isBulletPoint(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	// Check for various bullet point markers
	bulletMarkers := []string{"• ", "* ", "- ", "+ ", "◦ ", "▪ ", "▫ "}

	for _, marker := range bulletMarkers {
		if strings.HasPrefix(trimmed, marker) {
			return true
		}
	}

	return false
}

// formatBulletLine handles bullet point formatting with proper indentation
func (m Model) formatBulletLine(line string, width int) []string {
	trimmed := strings.TrimSpace(line)

	// Find the bullet marker and content
	var marker, content string
	bulletMarkers := []string{"• ", "* ", "- ", "+ ", "◦ ", "▪ ", "▫ "}

	for _, bm := range bulletMarkers {
		if strings.HasPrefix(trimmed, bm) {
			marker = bm
			content = strings.TrimSpace(trimmed[len(bm):])
			break
		}
	}

	if marker == "" {
		// Fallback to regular text wrapping
		return m.wrapRegularText(line, width)
	}

	// Calculate indentation for continuation lines
	indent := strings.Repeat(" ", len(marker))
	availableWidth := width - len(marker)

	if availableWidth <= 0 {
		return []string{line}
	}

	// Wrap the content
	wrappedContent := m.wrapRegularText(content, availableWidth)

	var result []string
	for i, contentLine := range wrappedContent {
		if i == 0 {
			// First line gets the bullet marker
			result = append(result, marker+contentLine)
		} else {
			// Continuation lines get indented
			result = append(result, indent+contentLine)
		}
	}

	return result
}

// isNumberedList checks if a line is part of a numbered list
func (m Model) isNumberedList(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	// Check for numbered list pattern (number followed by . or ))
	for i, r := range trimmed {
		if r >= '0' && r <= '9' {
			continue
		} else if r == '.' || r == ')' {
			// Found a number followed by . or ), check if there's a space after
			if i > 0 && i+1 < len(trimmed) && trimmed[i+1] == ' ' {
				return true
			}
			break
		} else {
			break
		}
	}

	return false
}

// formatNumberedLine handles numbered list formatting
func (m Model) formatNumberedLine(line string, width int) []string {
	trimmed := strings.TrimSpace(line)

	// Find the number and content
	var prefix, content string
	for i, r := range trimmed {
		if r >= '0' && r <= '9' {
			continue
		} else if r == '.' || r == ')' {
			if i > 0 && i+1 < len(trimmed) && trimmed[i+1] == ' ' {
				prefix = trimmed[:i+2] // Include the number, delimiter, and space
				content = strings.TrimSpace(trimmed[i+2:])
				break
			}
		} else {
			break
		}
	}

	if prefix == "" {
		// Fallback to regular text wrapping
		return m.wrapRegularText(line, width)
	}

	// Calculate indentation for continuation lines
	indent := strings.Repeat(" ", len(prefix))
	availableWidth := width - len(prefix)

	if availableWidth <= 0 {
		return []string{line}
	}

	// Wrap the content
	wrappedContent := m.wrapRegularText(content, availableWidth)

	var result []string
	for i, contentLine := range wrappedContent {
		if i == 0 {
			// First line gets the number prefix
			result = append(result, prefix+contentLine)
		} else {
			// Continuation lines get indented
			result = append(result, indent+contentLine)
		}
	}

	return result
}

// wrapRegularText performs word-wrapping on regular text with markdown formatting
func (m Model) wrapRegularText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	// Apply markdown formatting first
	formattedText := m.parseMarkdownFormatting(text)

	// For wrapped text with styling, we need to handle it line by line
	// since lipgloss styles can contain ANSI escape codes that affect visual length
	// but not the actual character count for wrapping purposes

	// For simplicity in this implementation, we'll work with the original text
	// for length calculations but apply formatting to each final line
	originalWords := strings.Fields(text)
	if len(originalWords) == 0 {
		return []string{""}
	}

	// Optimization for short texts that don't need wrapping
	if len(text) <= width {
		return []string{formattedText}
	}

	// Preallocate the result slice based on an estimate
	estimatedLines := (len(text) / width) + 1
	lines := make([]string, 0, estimatedLines)

	// Use strings.Builder for better performance
	var sb strings.Builder

	for _, word := range originalWords {
		// Check if adding this word would exceed the width
		if sb.Len() == 0 {
			sb.WriteString(word)
		} else if sb.Len()+len(word)+1 <= width {
			sb.WriteString(" ")
			sb.WriteString(word)
		} else {
			// Line is full, apply formatting and append it, then start a new one
			lineText := sb.String()
			formattedLine := m.parseMarkdownFormatting(lineText)
			lines = append(lines, formattedLine)
			sb.Reset()
			sb.WriteString(word)
		}
	}

	// Add the last line if there's anything left
	if sb.Len() > 0 {
		lineText := sb.String()
		formattedLine := m.parseMarkdownFormatting(lineText)
		lines = append(lines, formattedLine)
	}

	return lines
}

// parseMarkdownFormatting processes text to apply markdown formatting like **bold** and _italic_
func (m Model) parseMarkdownFormatting(text string) string {
	result := text

	// Process **bold** text markers first
	for {
		start := strings.Index(result, "**")
		if start == -1 {
			break
		}

		// Find the closing **
		end := strings.Index(result[start+2:], "**")
		if end == -1 {
			// No closing **, leave as is
			break
		}

		// Adjust end position to be relative to the full string
		end = start + 2 + end

		// Extract the text between ** markers
		boldText := result[start+2 : end]

		// Apply bold styling
		styledText := m.styles.boldText.Render(boldText)

		// Replace the **text** with styled text
		result = result[:start] + styledText + result[end+2:]
	}

	// Process _italic_ text markers
	for {
		start := strings.Index(result, "_")
		if start == -1 {
			break
		}

		// Find the closing _
		end := strings.Index(result[start+1:], "_")
		if end == -1 {
			// No closing _, leave as is
			break
		}

		// Adjust end position to be relative to the full string
		end = start + 1 + end

		// Extract the text between _ markers
		italicText := result[start+1 : end]

		// Apply italic styling
		styledText := m.styles.italicText.Render(italicText)

		// Replace the _text_ with styled text
		result = result[:start] + styledText + result[end+1:]
	}

	return result
}

// renderStatusBar renders the status bar showing model and token information
func (m *Model) renderStatusBar() string {
	// Use the pre-defined style from styles.go
	statusStyle := m.styles.statusBar.Width(m.width - 2)

	// Get model name
	modelInfo := fmt.Sprintf("Model: %s", m.config.ChatModel)

	// Get context window size efficiently (NEVER make API calls during render!)
	contextSize := m.getCachedModelContextSize()
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

// getCachedModelContextSize returns the context size for the current model,
// using ONLY cached/fallback values to ensure zero latency during rendering
func (m *Model) getCachedModelContextSize() int {
	// Check if we have the context size cached for the current model
	if m.cachedModelName == m.config.ChatModel && m.cachedContextSize > 0 {
		return m.cachedContextSize
	}

	// ALWAYS use fallback values for immediate rendering (NEVER make API calls!)
	// This ensures UI responsiveness at all times
	contextSize := fallbackContextSize(m.config.ChatModel)

	// Cache the fallback result immediately
	m.cachedModelName = m.config.ChatModel
	m.cachedContextSize = contextSize

	return contextSize
}

// GetRAGService returns the RAG service for external access
func (m Model) GetRAGService() *rag.Service {
	return m.ragService
}

// executeApprovedTool executes the pending tool with user approval
func (m Model) executeApprovedTool(updateTrustLevel bool) (tea.Model, tea.Cmd) {
	if m.pendingToolPermission == nil {
		return m, nil
	}

	// If user chose to trust for session, update the trust level
	if updateTrustLevel {
		_ = m.config.SetToolTrustLevel(m.pendingToolPermission.ToolName, 2) // TrustSession
	}

	// Execute the tool using the unified tool system
	result, err := tooling.DefaultRegistry.ExecuteTool(m.pendingToolPermission.ToolCall.Function.Name, m.pendingToolPermission.ToolCall.Function.Arguments)
	if err != nil {
		// Add error message using current conversation ULID
		errorMsg := Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Error executing %s: %v", m.pendingToolPermission.ToolName, err),
			Time:    time.Now(),
			ULID:    m.currentConversationULID, // Use conversation ULID for traceability
		}
		m.messages = append(m.messages, errorMsg)

		// Log tool execution error with conversation ULID
		logConversationEvent(m.currentConversationULID, "assistant", errorMsg.Content, m.config.ChatModel)
	} else {
		// Add successful result
		var resultStr string
		switch v := result.(type) {
		case string:
			resultStr = v
		default:
			resultStr = fmt.Sprintf("%v", result)
		}

		resultMsg := Message{
			Role:    "assistant",
			Content: fmt.Sprintf("✅ Tool '%s' executed successfully:\n%s", m.pendingToolPermission.ToolName, resultStr),
			Time:    time.Now(),
			ULID:    m.currentConversationULID, // Use conversation ULID for traceability
		}
		m.messages = append(m.messages, resultMsg)

		// Log tool execution result with conversation ULID
		logConversationEvent(m.currentConversationULID, "assistant", resultMsg.Content, m.config.ChatModel)
	}

	// Clear permission state
	m.pendingToolPermission = nil
	m.waitingForPermission = false
	m.inputModel.SetPlaceholder("Type your question...")

	// Update UI
	m.messagesNeedsUpdate = true
	m.messageCache.InvalidateCache()

	return m, nil
} // denyToolExecution denies the pending tool execution
func (m Model) denyToolExecution() (tea.Model, tea.Cmd) {
	if m.pendingToolPermission == nil {
		return m, nil
	}

	// Add denial message using current conversation ULID
	denialMsg := Message{
		Role:    "assistant",
		Content: fmt.Sprintf("❌ Tool '%s' execution denied by user", m.pendingToolPermission.ToolName),
		Time:    time.Now(),
		ULID:    m.currentConversationULID, // Use conversation ULID for traceability
	}
	m.messages = append(m.messages, denialMsg)

	// Log tool denial with conversation ULID
	logConversationEvent(m.currentConversationULID, "assistant", denialMsg.Content, m.config.ChatModel)

	// Clear permission state
	m.pendingToolPermission = nil
	m.waitingForPermission = false
	m.inputModel.SetPlaceholder("Type your question...")

	// Update UI
	m.messagesNeedsUpdate = true
	m.messageCache.InvalidateCache()

	return m, nil
}

// UpdateFromConfiguration updates the session system prompt from configuration changes
// but only if the session prompt has not been manually modified
func (m *Model) UpdateFromConfiguration(newConfig *configuration.Config) {
	logger := logging.WithComponent("chat")

	// Always log when this method is called
	logger.Info("UpdateFromConfiguration called",
		"old_rag_enabled", m.config.RAGEnabled,
		"new_rag_enabled", newConfig.RAGEnabled,
		"old_chromadb_url", m.config.ChromaDBURL,
		"new_chromadb_url", newConfig.ChromaDBURL,
		"old_embedding_model", m.config.EmbeddingModel,
		"new_embedding_model", newConfig.EmbeddingModel,
	)

	// Check if RAG-related settings have changed
	ragSettingsChanged := m.config.RAGEnabled != newConfig.RAGEnabled ||
		m.config.ChromaDBURL != newConfig.ChromaDBURL ||
		m.config.EmbeddingModel != newConfig.EmbeddingModel ||
		m.config.ChromaDBDistance != newConfig.ChromaDBDistance ||
		m.config.MaxDocuments != newConfig.MaxDocuments ||
		!equalStringMaps(m.config.SelectedCollections, newConfig.SelectedCollections)

	logger.Info("RAG settings change check",
		"ragSettingsChanged", ragSettingsChanged,
		"rag_enabled_changed", m.config.RAGEnabled != newConfig.RAGEnabled,
		"chromadb_url_changed", m.config.ChromaDBURL != newConfig.ChromaDBURL,
		"embedding_model_changed", m.config.EmbeddingModel != newConfig.EmbeddingModel,
	)

	if ragSettingsChanged {
		logger.Info("RAG configuration changed, updating RAG service",
			"old_rag_enabled", m.config.RAGEnabled,
			"new_rag_enabled", newConfig.RAGEnabled,
			"old_chromadb_url", m.config.ChromaDBURL,
			"new_chromadb_url", newConfig.ChromaDBURL,
			"old_embedding_model", m.config.EmbeddingModel,
			"new_embedding_model", newConfig.EmbeddingModel,
		)

		// Update the configuration reference first
		m.config = newConfig

		// Update RAG service configuration
		if m.ragService != nil {
			// Update the RAG service's configuration reference to use the new config
			m.ragService.UpdateConfig(newConfig)

			// Update selected collections
			m.ragService.UpdateSelectedCollections(newConfig.SelectedCollections)

			// If RAG is enabled, reinitialize the service to pick up new settings
			if newConfig.RAGEnabled {
				go func() {
					err := m.ragService.Initialize(m.ctx)
					if err != nil {
						logger.Warn("Failed to reinitialize RAG service after configuration change", "error", err.Error())
					} else {
						logger.Info("RAG service successfully reinitialized after configuration change")
					}
				}()
			}
		}
	} else {
		// Update the configuration reference
		m.config = newConfig
	}

	// Only update session system prompt if it has not been manually modified
	if !m.sessionSystemPromptManual {
		if m.sessionSystemPrompt != newConfig.DefaultSystemPrompt {
			logger.Info("Updating session system prompt from configuration change",
				"old_prompt_length", len(m.sessionSystemPrompt),
				"new_prompt_length", len(newConfig.DefaultSystemPrompt),
				"manual_override", m.sessionSystemPromptManual)

			m.sessionSystemPrompt = newConfig.DefaultSystemPrompt
			m.systemPromptNeedsUpdate = true
		}
	}
}

// equalStringMaps compares two map[string]bool for equality
func equalStringMaps(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// formatConversationHistory formats the conversation history as a readable text string for copying
func (m *Model) formatConversationHistory() string {
	if len(m.messages) == 0 {
		return "No conversation history available."
	}

	var sb strings.Builder
	sb.WriteString("Conversation History\n")
	sb.WriteString("==================\n\n")

	for i, msg := range m.messages {
		// Skip hidden messages (like tool responses)
		if msg.Hidden {
			continue
		}

		// Format timestamp
		timestamp := msg.Time.Format("2006-01-02 15:04:05")

		// Format role
		role := strings.ToUpper(msg.Role)

		// Write header
		sb.WriteString(fmt.Sprintf("[%s] %s:\n", timestamp, role))

		// Handle tool messages differently
		if msg.Role == "tool" && msg.ToolName != "" {
			sb.WriteString(fmt.Sprintf("Tool: %s\n", msg.ToolName))
		}

		// Write content with proper indentation
		content := strings.TrimSpace(msg.Content)
		if content != "" {
			// Indent content for readability
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				sb.WriteString("  " + line + "\n")
			}
		}

		// Add separator between messages (except for the last one)
		if i < len(m.messages)-1 {
			sb.WriteString("\n---\n\n")
		}
	}

	sb.WriteString("\n\nEnd of conversation history.")
	return sb.String()
}

// copyConversationToClipboard copies the formatted conversation history to the system clipboard
func (m *Model) copyConversationToClipboard() error {
	formattedHistory := m.formatConversationHistory()
	return clipboard.WriteAll(formattedHistory)
}

// UpdateAgentsFile updates the AGENTS.md file for the chat model and refreshes the system prompt
func (m *Model) UpdateAgentsFile(agentsFile *agents.AgentsFile) {
	logger := logging.WithComponent("chat")

	// Store old and new states for logging
	oldFile := m.agentsFile
	m.agentsFile = agentsFile

	if oldFile == nil && agentsFile != nil {
		logger.Info("AGENTS.md file added to chat model",
			"path", agentsFile.Path,
			"size_chars", len(agentsFile.Content))
	} else if oldFile != nil && agentsFile == nil {
		logger.Info("AGENTS.md file removed from chat model",
			"old_path", oldFile.Path)
	} else if oldFile != nil && agentsFile != nil {
		logger.Info("AGENTS.md file updated in chat model",
			"old_path", oldFile.Path,
			"new_path", agentsFile.Path,
			"old_size_chars", len(oldFile.Content),
			"new_size_chars", len(agentsFile.Content))
	} else {
		logger.Debug("AGENTS.md file update called but no change (both nil)")
	}

	// Rebuild the session system prompt if it hasn't been manually modified
	if !m.sessionSystemPromptManual {
		basePrompt := m.config.DefaultSystemPrompt
		if agentsFile != nil {
			m.sessionSystemPrompt = basePrompt + agentsFile.FormatAsSystemPromptAddition()
			logger.Info("Updated session system prompt with AGENTS.md content",
				"base_prompt_length", len(basePrompt),
				"agents_content_length", len(agentsFile.Content),
				"total_prompt_length", len(m.sessionSystemPrompt))
		} else {
			m.sessionSystemPrompt = basePrompt
			logger.Info("Updated session system prompt without AGENTS.md content",
				"prompt_length", len(m.sessionSystemPrompt))
		}
		m.systemPromptNeedsUpdate = true
	} else {
		logger.Debug("Session system prompt manually modified, not updating with AGENTS.md changes")
	}
}
