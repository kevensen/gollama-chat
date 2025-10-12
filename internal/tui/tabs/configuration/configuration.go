package configuration

import (
	"fmt"
	"maps"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/configuration/models"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/configuration/utils/connection"
	ragTab "github.com/kevensen/gollama-chat/internal/tui/tabs/rag"
)

// ConfigUpdatedMsg is sent when the configuration has been updated and saved
type ConfigUpdatedMsg = ragTab.ConfigUpdatedMsg

// Field represents a configuration field being edited
type Field int

const (
	ChatModelField Field = iota
	DefaultSystemPromptField
	OllamaURLField
	LogLevelField
	EnableFileLoggingField
	AgentsFileEnabledField
)

// Model represents the configuration tab model
type Model struct {
	config                 *configuration.Config
	editConfig             *configuration.Config // Copy for editing
	activeField            Field
	editing                bool
	input                  string
	cursor                 int
	width                  int
	height                 int
	message                string
	messageStyle           lipgloss.Style
	ollamaStatus           connection.Status
	chromaDBStatus         connection.Status
	modelPanel             models.Model
	showModelPanel         bool
	showSystemPromptPanel  bool   // Whether the system prompt editing panel is visible
	systemPromptEditInput  string // Input text for system prompt editing
	systemPromptEditCursor int    // Cursor position in system prompt editing
	systemPromptEditMode   bool   // Whether the system prompt panel is in edit mode
	systemPromptScrollY    int    // Vertical scroll position in the system prompt panel
}

// NewModel creates a new configuration model
func NewModel(config *configuration.Config) Model {
	logger := logging.WithComponent("configuration_tab")
	logger.Info("Creating new configuration model", "ollama_url", config.OllamaURL, "chromadb_url", config.ChromaDBURL, "rag_enabled", config.RAGEnabled)

	// Get system prompt from file
	systemPrompt, err := config.GetSystemPrompt()
	if err != nil {
		logger.Error("Failed to load system prompt", "error", err)
		systemPrompt = "" // Fall back to empty string
	}

	// Create a copy for editing
	editConfig := &configuration.Config{
		ChatModel:           config.ChatModel,
		OllamaURL:           config.OllamaURL,
		DefaultSystemPrompt: systemPrompt, // Use system prompt from file
		ToolTrustLevels:     make(map[string]int),
		MCPServers:          make([]configuration.MCPServer, len(config.MCPServers)),
		LogLevel:            config.LogLevel,
		EnableFileLogging:   config.EnableFileLogging,
		AgentsFileEnabled:   config.AgentsFileEnabled,
	}

	// Copy the toolTrustLevels map
	maps.Copy(editConfig.ToolTrustLevels, config.ToolTrustLevels)

	// Copy the MCPServers slice
	copy(editConfig.MCPServers, config.MCPServers)

	return Model{
		config:                 config,
		editConfig:             editConfig,
		activeField:            OllamaURLField,
		editing:                false,
		ollamaStatus:           connection.StatusUnknown,
		chromaDBStatus:         connection.StatusUnknown,
		modelPanel:             models.NewModel(),
		showModelPanel:         false,
		showSystemPromptPanel:  false,
		systemPromptEditInput:  "",
		systemPromptEditCursor: 0,
		systemPromptEditMode:   false,
		systemPromptScrollY:    0,
		messageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true),
	}
}

// Init initializes the configuration model
func (m Model) Init() tea.Cmd {
	logger := logging.WithComponent("configuration_tab")
	logger.Info("Initializing configuration tab, starting connection checks",
		"ollama_url", m.editConfig.OllamaURL,
		"chromadb_url", m.editConfig.ChromaDBURL)

	// Start checking connections when the model initializes
	return tea.Batch(
		connection.OllamaStatus(m.editConfig.OllamaURL),
		connection.ChromaDBStatus(m.editConfig.ChromaDBURL),
	)
}

// Update handles messages and updates the configuration model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update model panel size - allocate about 1/3 of width for the panel
		panelWidth := m.width / 3
		// Use the same height calculation as the main settings box
		panelHeight := m.height + 2
		m.modelPanel = m.modelPanel.SetSize(panelWidth, panelHeight)

	case connection.CheckMsg:
		logger := logging.WithComponent("configuration_tab")
		switch msg.Server {
		case "ollama":
			logger.Info("Ollama connection check completed",
				"url", m.editConfig.OllamaURL,
				"full_url", msg.FullURL,
				"status", msg.Status,
				"error", msg.Error)
			m.ollamaStatus = msg.Status
			if msg.Error != nil {
				logger.Warn("Ollama connection failed", "url", m.editConfig.OllamaURL, "full_url", msg.FullURL, "error", msg.Error)
			}
		case "chromadb":
			logger.Info("ChromaDB connection check completed",
				"url", m.editConfig.ChromaDBURL,
				"full_url", msg.FullURL,
				"status", msg.Status,
				"error", msg.Error)
			m.chromaDBStatus = msg.Status
			if msg.Error != nil {
				logger.Warn("ChromaDB connection failed", "url", m.editConfig.ChromaDBURL, "full_url", msg.FullURL, "error", msg.Error)
			}
		}

	case models.FetchModelsMsg:
		var cmd tea.Cmd
		m.modelPanel, cmd = m.modelPanel.Update(msg)
		return m, cmd

	case models.ModelSelectedMsg:
		// Handle model selection
		switch msg.Mode {
		case models.ChatModelSelection:
			m.editConfig.ChatModel = msg.ModelName
		case models.EmbeddingModelSelection:
			m.editConfig.EmbeddingModel = msg.ModelName
		}
		m.showModelPanel = false

		// Auto-save the configuration after model selection
		if updateCmd, saveErr := m.autoSaveConfiguration(); saveErr != nil {
			m.message = fmt.Sprintf("Selected: %s (save failed: %s)", msg.ModelName, saveErr.Error())
			m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for warning
			return m, nil
		} else {
			m.message = fmt.Sprintf("Selected and saved: %s", msg.ModelName)
			m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))
			return m, updateCmd
		}

	case ConfigUpdatedMsg:
		// Handle configuration updates from other tabs (like MCP)
		logger := logging.WithComponent("configuration_tab")
		logger.Debug("Received configuration update from another tab")

		// Update our main config reference
		m.config = msg.Config

		// Get system prompt from file for the updated config
		systemPrompt, err := msg.Config.GetSystemPrompt()
		if err != nil {
			logger.Error("Failed to load system prompt during config update", "error", err)
			systemPrompt = "" // Fall back to empty string
		}

		// Update editConfig to reflect the new state, but preserve any current edits
		// This ensures that if the user is currently editing fields, their changes are preserved
		// while still picking up changes from other tabs (like MCP server changes)
		if !m.editing {
			// If not currently editing, update editConfig completely
			m.editConfig = &configuration.Config{
				ChatModel:           msg.Config.ChatModel,
				EmbeddingModel:      msg.Config.EmbeddingModel,
				RAGEnabled:          msg.Config.RAGEnabled,
				OllamaURL:           msg.Config.OllamaURL,
				ChromaDBURL:         msg.Config.ChromaDBURL,
				ChromaDBDistance:    msg.Config.ChromaDBDistance,
				MaxDocuments:        msg.Config.MaxDocuments,
				SelectedCollections: make(map[string]bool),
				DefaultSystemPrompt: systemPrompt, // Use system prompt from file
				ToolTrustLevels:     make(map[string]int),
				MCPServers:          make([]configuration.MCPServer, len(msg.Config.MCPServers)),
				LogLevel:            msg.Config.LogLevel,
				EnableFileLogging:   msg.Config.EnableFileLogging,
				AgentsFileEnabled:   msg.Config.AgentsFileEnabled,
			}

			// Copy the maps and slice
			maps.Copy(m.editConfig.SelectedCollections, msg.Config.SelectedCollections)
			maps.Copy(m.editConfig.ToolTrustLevels, msg.Config.ToolTrustLevels)
			copy(m.editConfig.MCPServers, msg.Config.MCPServers)
		} else {
			// If currently editing, only update non-UI fields (like MCP servers)
			// but preserve the current edit state for UI fields
			maps.Copy(m.editConfig.SelectedCollections, msg.Config.SelectedCollections)
			maps.Copy(m.editConfig.ToolTrustLevels, msg.Config.ToolTrustLevels)
			m.editConfig.MCPServers = make([]configuration.MCPServer, len(msg.Config.MCPServers))
			copy(m.editConfig.MCPServers, msg.Config.MCPServers)
		}

	case tea.KeyMsg:
		if m.editing {
			return m.handleEditingKeys(msg)
		} else {
			return m.handleNavigationKeys(msg)
		}
	}

	return m, nil
}

// handleNavigationKeys handles keys when not editing a field
func (m Model) handleNavigationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If model panel is visible, forward keys to it first
	if m.showModelPanel {
		switch msg.String() {
		case "esc":
			// Close model panel
			m.showModelPanel = false
			return m, nil
		default:
			// Forward to model panel
			var cmd tea.Cmd
			m.modelPanel, cmd = m.modelPanel.Update(msg)
			return m, cmd
		}
	}

	// If system prompt panel is visible, handle it
	if m.showSystemPromptPanel {
		// Add debug logging for ALL keys received in system prompt panel
		logger := logging.WithComponent("system_prompt_keys")
		logger.Debug("Key received in system prompt panel", "key", msg.String(), "type", msg.Type, "edit_mode", m.systemPromptEditMode)

		switch msg.String() {
		case "esc":
			// Close system prompt panel
			m.showSystemPromptPanel = false
			m.systemPromptEditInput = ""
			m.systemPromptEditCursor = 0
			m.systemPromptEditMode = false
			m.systemPromptScrollY = 0
			return m, nil
		case "ctrl+e":
			// Enable edit mode
			if !m.systemPromptEditMode {
				m.systemPromptEditMode = true
				m.systemPromptEditInput = m.editConfig.DefaultSystemPrompt
				m.systemPromptEditCursor = 0 // Start at beginning of text
				m.systemPromptScrollY = 0    // Reset scroll when entering edit mode
			}
			return m, nil
		case "ctrl+s":
			// Save changes (only if in edit mode)
			if m.systemPromptEditMode {
				m.editConfig.DefaultSystemPrompt = m.systemPromptEditInput
				m.systemPromptEditMode = false

				// Save system prompt to file using the new method
				if err := m.config.SetSystemPrompt(m.systemPromptEditInput); err != nil {
					m.message = fmt.Sprintf("System prompt save failed: %s", err.Error())
					m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for warning
					return m, nil
				}

				// System prompt saved successfully - no need to save main config since it's in a separate file
				m.message = "System prompt updated and saved"
				m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))

				// Send config update message to notify other tabs that the system prompt changed
				updateCmd := func() tea.Msg {
					return ConfigUpdatedMsg{Config: m.config}
				}
				return m, updateCmd
			}
			return m, nil
		}

		// Handle scrolling in both edit and view modes (process BEFORE text editing)
		switch msg.String() {
		case "pgup", "page_up", "ctrl+u":
			// Scroll up in the panel
			oldScrollY := m.systemPromptScrollY
			if m.systemPromptScrollY > 0 {
				m.systemPromptScrollY -= 5
				if m.systemPromptScrollY < 0 {
					m.systemPromptScrollY = 0
				}
			}
			logger.Debug("Page up scrolling", "old_scroll", oldScrollY, "new_scroll", m.systemPromptScrollY)
			return m, nil
		case "pgdown", "page_down", "ctrl+d":
			// Scroll down in the panel - calculate max scroll first
			textAreaHeight := m.height - 12
			if textAreaHeight < 5 {
				textAreaHeight = 5
			}

			textWidth := (m.width / 3) - 6
			if textWidth < 20 {
				textWidth = 20
			}

			var displayText string
			if m.systemPromptEditMode {
				displayText = m.systemPromptEditInput
			} else {
				displayText = m.editConfig.DefaultSystemPrompt
			}

			displayTextWithIndicators := m.renderVisibleChars(displayText)
			lines := m.wrapText(displayTextWithIndicators, textWidth)

			maxScroll := len(lines) - textAreaHeight
			if maxScroll < 0 {
				maxScroll = 0
			}

			oldScrollY := m.systemPromptScrollY
			m.systemPromptScrollY += 5
			if m.systemPromptScrollY > maxScroll {
				m.systemPromptScrollY = maxScroll
			}

			logger.Debug("Page down scrolling",
				"old_scroll", oldScrollY,
				"new_scroll", m.systemPromptScrollY,
				"max_scroll", maxScroll,
				"total_lines", len(lines),
				"text_area_height", textAreaHeight)
			return m, nil
		}

		// Only allow text editing if in edit mode
		if m.systemPromptEditMode {
			switch msg.String() {
			case "backspace":
				if m.systemPromptEditCursor > 0 {
					runes := []rune(m.systemPromptEditInput)
					if m.systemPromptEditCursor <= len(runes) {
						// Remove the rune before cursor
						newRunes := append(runes[:m.systemPromptEditCursor-1], runes[m.systemPromptEditCursor:]...)
						m.systemPromptEditInput = string(newRunes)
						m.systemPromptEditCursor--
					}
				}
				return m, nil
			case "delete":
				runes := []rune(m.systemPromptEditInput)
				if m.systemPromptEditCursor < len(runes) {
					// Remove the rune at cursor
					newRunes := append(runes[:m.systemPromptEditCursor], runes[m.systemPromptEditCursor+1:]...)
					m.systemPromptEditInput = string(newRunes)
				}
				return m, nil
			case "left":
				if m.systemPromptEditCursor > 0 {
					m.systemPromptEditCursor--
				}
				return m, nil
			case "right":
				runes := []rune(m.systemPromptEditInput)
				if m.systemPromptEditCursor < len(runes) {
					m.systemPromptEditCursor++
				}
				return m, nil
			case "up":
				// Move cursor up one line
				m.systemPromptEditCursor = m.moveCursorUp(m.systemPromptEditInput, m.systemPromptEditCursor)
				m.ensureCursorVisible()
				return m, nil
			case "down":
				// Move cursor down one line
				m.systemPromptEditCursor = m.moveCursorDown(m.systemPromptEditInput, m.systemPromptEditCursor)
				m.ensureCursorVisible()
				return m, nil
			case "home":
				// Move to beginning of current line
				m.systemPromptEditCursor = m.moveCursorToLineStart(m.systemPromptEditInput, m.systemPromptEditCursor)
				return m, nil
			case "end":
				// Move to end of current line
				m.systemPromptEditCursor = m.moveCursorToLineEnd(m.systemPromptEditInput, m.systemPromptEditCursor)
				return m, nil
			case "ctrl+home":
				// Move to beginning of text
				m.systemPromptEditCursor = 0
				return m, nil
			case "ctrl+end":
				// Move to end of text
				runes := []rune(m.systemPromptEditInput)
				m.systemPromptEditCursor = len(runes)
				return m, nil
			case "enter":
				// Insert newline
				runes := []rune(m.systemPromptEditInput)
				newRunes := append(runes[:m.systemPromptEditCursor], append([]rune{'\n'}, runes[m.systemPromptEditCursor:]...)...)
				m.systemPromptEditInput = string(newRunes)
				m.systemPromptEditCursor++
				return m, nil
			case "tab":
				// Insert tab or spaces
				tabRunes := []rune("    ") // 4 spaces
				runes := []rune(m.systemPromptEditInput)
				newRunes := append(runes[:m.systemPromptEditCursor], append(tabRunes, runes[m.systemPromptEditCursor:]...)...)
				m.systemPromptEditInput = string(newRunes)
				m.systemPromptEditCursor += len(tabRunes)
				return m, nil
			default:
				// Handle regular character input
				if len(msg.String()) == 1 {
					char := msg.String()
					// Allow all printable characters including space
					if char >= " " && char <= "~" || char >= "\u00A0" { // Printable ASCII + extended
						runes := []rune(m.systemPromptEditInput)
						charRunes := []rune(char)
						newRunes := append(runes[:m.systemPromptEditCursor], append(charRunes, runes[m.systemPromptEditCursor:]...)...)
						m.systemPromptEditInput = string(newRunes)
						m.systemPromptEditCursor += len(charRunes)
					}
				}
				return m, nil
			}
		}

		// If not in edit mode, ignore text input keys
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.activeField > 0 {
			m.activeField--
		}

	case "down", "j":
		if m.activeField < AgentsFileEnabledField { // Updated to use actual last field
			m.activeField++
		}

	case "ctrl+u", "page_up":
		// Page up functionality removed for system prompt since it no longer expands

	case "ctrl+d", "page_down":
		// Page down functionality removed for system prompt since it no longer expands

	case "home":
		// Go to first field
		m.activeField = ChatModelField

	case "end":
		// Go to last field
		m.activeField = AgentsFileEnabledField

	case "enter", " ":
		// Check if we should show model selection panel
		if m.activeField == ChatModelField {
			if m.ollamaStatus == connection.StatusConnected {
				var mode models.SelectionMode
				if m.activeField == ChatModelField {
					mode = models.ChatModelSelection
				} else {
					mode = models.EmbeddingModelSelection
				}

				m.modelPanel = m.modelPanel.SetVisible(true, mode)
				m.showModelPanel = true

				// Fetch models from Ollama
				return m, models.FetchModels(m.editConfig.OllamaURL)
			} else {
				m.message = "Cannot fetch models: Ollama server not connected"
				m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("9"))
			}
		} else if m.activeField == DefaultSystemPromptField {
			// Open system prompt viewing panel (read-only initially)
			m.showSystemPromptPanel = true
			m.systemPromptEditInput = m.editConfig.DefaultSystemPrompt
			m.systemPromptEditCursor = 0
			m.systemPromptEditMode = false
			m.systemPromptScrollY = 0
		} else if m.activeField == EnableFileLoggingField || m.activeField == AgentsFileEnabledField {
			// Toggle boolean fields directly
			logger := logging.WithComponent("configuration_tab")
			fieldName := m.getFieldName(m.activeField)
			var oldValue, newValue bool

			switch m.activeField {
			case EnableFileLoggingField:
				oldValue = m.editConfig.EnableFileLogging
				m.editConfig.EnableFileLogging = !m.editConfig.EnableFileLogging
				newValue = m.editConfig.EnableFileLogging
			case AgentsFileEnabledField:
				oldValue = m.editConfig.AgentsFileEnabled
				m.editConfig.AgentsFileEnabled = !m.editConfig.AgentsFileEnabled
				newValue = m.editConfig.AgentsFileEnabled
				// Add specific logging for AGENTS.md configuration change
				if newValue {
					logger.Info("AGENTS.md detection enabled via configuration UI")
				} else {
					logger.Info("AGENTS.md detection disabled via configuration UI")
				}
			}

			logger.Info("Boolean field toggled", "field", fieldName, "old_value", oldValue, "new_value", newValue)

			// Auto-save the configuration after toggle
			if updateCmd, saveErr := m.autoSaveConfiguration(); saveErr != nil {
				m.message = fmt.Sprintf("Field toggled but save failed: %s", saveErr.Error())
				m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for warning
				return m, nil
			} else {
				m.message = fmt.Sprintf("Field toggled to %t and saved", newValue)
				m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))
				return m, updateCmd
			}
		} else if m.activeField == LogLevelField {
			// Cycle through log levels
			logger := logging.WithComponent("configuration_tab")
			fieldName := m.getFieldName(m.activeField)
			oldValue := m.editConfig.LogLevel

			// Define the log levels in order
			logLevels := []string{"debug", "info", "warn", "error"}
			currentIndex := -1

			// Find current log level index
			for i, level := range logLevels {
				if level == oldValue {
					currentIndex = i
					break
				}
			}

			// Cycle to next level (wrap around to beginning if at end)
			nextIndex := (currentIndex + 1) % len(logLevels)
			newValue := logLevels[nextIndex]
			m.editConfig.LogLevel = newValue

			logger.Info("Log level cycled", "field", fieldName, "old_value", oldValue, "new_value", newValue)

			// Auto-save the configuration after changing log level
			if updateCmd, saveErr := m.autoSaveConfiguration(); saveErr != nil {
				m.message = fmt.Sprintf("Log level changed but save failed: %s", saveErr.Error())
				m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for warning
				return m, nil
			} else {
				m.message = fmt.Sprintf("Log level changed to '%s' and saved", newValue)
				m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))
				return m, updateCmd
			}
		} else {
			// Start editing the active field
			logger := logging.WithComponent("configuration_tab")
			fieldName := m.getFieldName(m.activeField)
			logger.Debug("Starting field edit", "field", fieldName, "current_value", m.getCurrentFieldValue())
			m.editing = true
			m.input = m.getCurrentFieldValue()
			m.cursor = len(m.input)
		}

	case "s", "S":
		// Save configuration
		return m.saveConfiguration()

	case "r", "R":
		// Reset to defaults
		m.editConfig = configuration.DefaultConfig()

		// Auto-save the default configuration
		if updateCmd, saveErr := m.autoSaveConfiguration(); saveErr != nil {
			m.message = fmt.Sprintf("Configuration reset to defaults (save failed: %s)", saveErr.Error())
			m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for warning
			return m, nil
		} else {
			m.message = "Configuration reset to defaults and saved"
			m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11"))
			return m, updateCmd
		}
	}

	return m, nil
}

// handleEditingKeys handles keys when editing a field
func (m Model) handleEditingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Save the field value
		if err := m.setCurrentFieldValue(m.input); err != nil {
			m.message = fmt.Sprintf("Error: %s", err.Error())
			m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("9"))
		} else {
			// Check if we should auto-save after field update
			// For URL fields, don't auto-save if basic required fields are missing
			shouldAutoSave := true
			if m.activeField == OllamaURLField && m.editConfig.ChatModel == "" {
				shouldAutoSave = false
				logger := logging.WithComponent("configuration_tab")
				logger.Debug("Skipping auto-save for URL field change due to missing required fields",
					"field", m.getFieldName(m.activeField), "missing_chat_model", m.editConfig.ChatModel == "")
			}

			var updateCmd tea.Cmd
			var saveErr error

			if shouldAutoSave {
				// Auto-save the configuration after successful field update
				updateCmd, saveErr = m.autoSaveConfiguration()
				if saveErr != nil {
					m.message = fmt.Sprintf("Field updated but save failed: %s", saveErr.Error())
					m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for warning
					m.editing = false
					return m, nil
				} else {
					m.message = "Field updated and saved"
					m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))
				}
			} else {
				m.message = "Field updated (not auto-saved due to incomplete configuration)"
				m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for info
			}

			// Check if we need to re-test connections
			var cmd tea.Cmd
			var cmds []tea.Cmd

			// Add the config update command if we auto-saved
			if shouldAutoSave && updateCmd != nil {
				cmds = append(cmds, updateCmd)
			}

			logger := logging.WithComponent("configuration_tab")
			switch m.activeField {
			case OllamaURLField:
				logger.Info("Triggering Ollama connection test after URL change", "new_url", m.editConfig.OllamaURL)
				m.ollamaStatus = connection.StatusChecking
				cmd = connection.OllamaStatus(m.editConfig.OllamaURL)
			}

			// Add the connection check command if there is one
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

			m.editing = false
			m.input = ""
			return m, tea.Batch(cmds...)
		}
		m.editing = false
		m.input = ""

	case "esc":
		// Cancel editing
		logger := logging.WithComponent("configuration_tab")
		fieldName := m.getFieldName(m.activeField)
		logger.Debug("Cancelled field edit", "field", fieldName)
		m.editing = false
		m.input = ""

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

	default:
		// Add character to input
		if len(msg.String()) == 1 {
			m.input = m.input[:m.cursor] + msg.String() + m.input[m.cursor:]
			m.cursor++
		}
	}

	return m, nil
}

// View renders the configuration tab
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// If model panel is visible, show side-by-side layout
	if m.showModelPanel {
		return m.renderSideBySideView()
	}

	// If system prompt panel is visible, show side-by-side layout
	if m.showSystemPromptPanel {
		return m.renderSystemPromptSideBySideView()
	}

	// Regular single-panel view
	return m.renderConfigurationView()
}

// renderSideBySideView renders configuration and model panel side by side
func (m Model) renderSideBySideView() string {
	// Allocate 2/3 width to config, 1/3 to model panel
	configWidth := (m.width * 2) / 3

	// Render configuration view with reduced width
	configView := m.renderConfigurationViewWithWidth(configWidth)

	// Render model panel
	modelPanelView := m.modelPanel.View()

	// Combine side by side
	return lipgloss.JoinHorizontal(lipgloss.Top, configView, modelPanelView)
}

// renderSystemPromptSideBySideView renders configuration and system prompt panel side by side
func (m Model) renderSystemPromptSideBySideView() string {
	// Allocate 2/3 width to config, 1/3 to system prompt panel
	configWidth := (m.width * 2) / 3
	panelWidth := m.width / 3

	// Render configuration view with reduced width
	configView := m.renderConfigurationViewWithWidth(configWidth)

	// Render system prompt panel - use same height calculation as main settings box
	systemPromptPanelView := m.renderSystemPromptPanel(panelWidth)

	// Combine side by side
	return lipgloss.JoinHorizontal(lipgloss.Top, configView, systemPromptPanelView)
}

// renderSystemPromptPanel renders the system prompt viewing/editing panel
func (m Model) renderSystemPromptPanel(width int) string {
	var content []string

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Align(lipgloss.Center)

	if m.systemPromptEditMode {
		content = append(content, titleStyle.Render("Edit System Prompt (Markdown)"))
	} else {
		content = append(content, titleStyle.Render("View System Prompt (Markdown)"))
	}
	content = append(content, "")

	// Text area content
	textAreaHeight := m.height - 12 // Leave room for title, help, border, etc.
	if textAreaHeight < 5 {
		textAreaHeight = 5
	}

	// Wrap the text to fit the available width
	textWidth := width - 6 // Account for padding and borders
	if textWidth < 20 {
		textWidth = 20
	}

	// Use the original text (with real newlines and tabs functioning)
	var displayText string
	if m.systemPromptEditMode {
		displayText = m.systemPromptEditInput
	} else {
		displayText = m.editConfig.DefaultSystemPrompt
		// Apply markdown syntax highlighting only in view mode
		displayText = m.applyMarkdownStyling(displayText)
	}

	// For display purposes, we'll render visible indicators but work with original text
	displayTextWithIndicators := m.renderVisibleChars(displayText)
	lines := m.wrapText(displayTextWithIndicators, textWidth)

	// Apply scrolling - limit scroll to prevent scrolling past content
	maxScroll := len(lines) - textAreaHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.systemPromptScrollY > maxScroll {
		m.systemPromptScrollY = maxScroll
	}

	// Get the visible lines based on scroll position
	startLine := m.systemPromptScrollY
	endLine := startLine + textAreaHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}

	var visibleLines []string
	if startLine < len(lines) {
		visibleLines = lines[startLine:endLine]
	}

	// Add cursor to the text (only if in edit mode and cursor is visible)
	displayLines := make([]string, len(visibleLines))
	copy(displayLines, visibleLines)

	if m.systemPromptEditMode {
		// Convert cursor position to display position
		displayCursor := m.convertCursorToDisplayPosition(m.systemPromptEditInput, m.systemPromptEditCursor)

		// Find which visual line the cursor should be on after wrapping
		charCount := 0

		for i, line := range lines {
			lineRunes := []rune(line)
			if charCount+len(lineRunes) >= displayCursor {
				// Check if this line is visible in the current viewport
				visibleLineIndex := i - m.systemPromptScrollY
				if visibleLineIndex >= 0 && visibleLineIndex < len(displayLines) {
					localCursor := displayCursor - charCount

					// Ensure cursor position is valid
					visibleLineRunes := []rune(visibleLines[visibleLineIndex])
					if localCursor > len(visibleLineRunes) {
						localCursor = len(visibleLineRunes)
					}

					// Add cursor to the display (insert mode - cursor goes between characters)
					if localCursor >= len(visibleLineRunes) {
						displayLines[visibleLineIndex] = visibleLines[visibleLineIndex] + "|"
					} else if localCursor >= 0 {
						lineRunes := []rune(visibleLines[visibleLineIndex])
						// Insert cursor between characters instead of replacing
						newLineRunes := append(lineRunes[:localCursor], append([]rune("|"), lineRunes[localCursor:]...)...)
						displayLines[visibleLineIndex] = string(newLineRunes)
					}
				}
				break
			}

			// Account for the line content and newlines
			charCount += len(lineRunes)
			if i < len(lines)-1 && charCount < len([]rune(displayTextWithIndicators)) {
				charCount++ // Account for the newline character
			}
		}
	} // Pad with empty lines if needed to fill the text area
	for len(displayLines) < textAreaHeight {
		displayLines = append(displayLines, "")
	}

	// Add the text area content
	content = append(content, displayLines...)

	// Calculate available space for help text positioning
	// Panel total height is m.height + 2, with border (2) and padding (2), so content area is m.height - 2
	// We want help text anchored at the very bottom of the content area
	totalContentHeight := m.height - 2
	
	// Count current content lines
	currentContentLines := len(content)
	
	// Prepare scroll indicator if there's more content
	var scrollInfo string
	hasScrollInfo := false
	if len(lines) > textAreaHeight {
		// Calculate the correct end line for display
		actualEndLine := min(m.systemPromptScrollY+textAreaHeight, len(lines))
		scrollInfo = fmt.Sprintf("Line %d-%d of %d",
			m.systemPromptScrollY+1,
			actualEndLine,
			len(lines))
		hasScrollInfo = true
	}
	
	// Calculate total lines needed for bottom elements
	helpLines := 2 // Two help text lines
	scrollLines := 0
	if hasScrollInfo {
		scrollLines = 1 // Just the scroll info line (no extra empty line)
	}
	
	// Calculate padding needed to position scroll info and help text at bottom
	bottomElementsLines := helpLines + scrollLines
	totalUsedLines := currentContentLines + bottomElementsLines
	paddingNeeded := totalContentHeight - totalUsedLines
	
	// Add padding to push bottom elements to the bottom
	if paddingNeeded > 0 {
		for i := 0; i < paddingNeeded; i++ {
			content = append(content, "")
		}
	}
	
	// Add scroll indicator just above help text (if needed)
	if hasScrollInfo {
		scrollStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Align(lipgloss.Right)
		content = append(content, scrollStyle.Render(scrollInfo))
	}

	// Help text at the bottom
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	if m.systemPromptEditMode {
		content = append(content, helpStyle.Render("PgUp/PgDn: Scroll • Ctrl+S: Save & Exit Edit Mode"))
		content = append(content, helpStyle.Render("Esc: Close Panel without Saving"))
	} else {
		content = append(content, helpStyle.Render("PgUp/PgDn: Scroll • Ctrl+E: Enter Edit Mode"))
		content = append(content, helpStyle.Render("Esc: Close Panel"))
	}

	// Apply border and styling - different border color for edit mode
	borderColor := "62"
	if m.systemPromptEditMode {
		borderColor = "10" // Green border when editing
	}

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(1, 1).
		Width(width - 2).
		Height(m.height + 2)

	return panelStyle.Render(strings.Join(content, "\n"))
}

// moveCursorUp moves the cursor up one line in the text
func (m Model) moveCursorUp(text string, cursor int) int {
	lines := strings.Split(text, "\n")

	// Find current line and column in runes
	currentRunePos := 0
	currentLine := 0
	currentCol := 0

	for i, line := range lines {
		lineRunes := []rune(line)
		if currentRunePos+len(lineRunes) >= cursor {
			currentLine = i
			currentCol = cursor - currentRunePos
			break
		}
		currentRunePos += len(lineRunes) + 1 // +1 for newline
	}

	// If already on first line, move to beginning
	if currentLine == 0 {
		return 0
	}

	// Move to previous line, trying to maintain column position
	prevLineRunes := []rune(lines[currentLine-1])
	if currentCol > len(prevLineRunes) {
		currentCol = len(prevLineRunes)
	}

	// Calculate new cursor position in runes
	newPos := 0
	for i := 0; i < currentLine-1; i++ {
		newPos += len([]rune(lines[i])) + 1
	}
	newPos += currentCol

	return newPos
}

// moveCursorDown moves the cursor down one line in the text
func (m Model) moveCursorDown(text string, cursor int) int {
	lines := strings.Split(text, "\n")

	// Find current line and column in runes
	currentRunePos := 0
	currentLine := 0
	currentCol := 0

	for i, line := range lines {
		lineRunes := []rune(line)
		if currentRunePos+len(lineRunes) >= cursor {
			currentLine = i
			currentCol = cursor - currentRunePos
			break
		}
		currentRunePos += len(lineRunes) + 1 // +1 for newline
	}

	// If already on last line, move to end
	if currentLine >= len(lines)-1 {
		totalRunes := 0
		for _, line := range lines {
			totalRunes += len([]rune(line))
		}
		totalRunes += len(lines) - 1 // Add newlines
		return totalRunes
	}

	// Move to next line, trying to maintain column position
	nextLineRunes := []rune(lines[currentLine+1])
	if currentCol > len(nextLineRunes) {
		currentCol = len(nextLineRunes)
	}

	// Calculate new cursor position in runes
	newPos := 0
	for i := 0; i <= currentLine; i++ {
		newPos += len([]rune(lines[i])) + 1
	}
	newPos += currentCol

	return newPos
}

// moveCursorToLineStart moves the cursor to the beginning of the current line
func (m Model) moveCursorToLineStart(text string, cursor int) int {
	lines := strings.Split(text, "\n")

	// Find current line
	currentRunePos := 0
	for _, line := range lines {
		lineRunes := []rune(line)
		if currentRunePos+len(lineRunes) >= cursor {
			return currentRunePos
		}
		currentRunePos += len(lineRunes) + 1 // +1 for newline
	}

	return cursor
}

// moveCursorToLineEnd moves the cursor to the end of the current line
func (m Model) moveCursorToLineEnd(text string, cursor int) int {
	lines := strings.Split(text, "\n")

	// Find current line
	currentRunePos := 0
	for _, line := range lines {
		lineRunes := []rune(line)
		if currentRunePos+len(lineRunes) >= cursor {
			return currentRunePos + len(lineRunes)
		}
		currentRunePos += len(lineRunes) + 1 // +1 for newline
	}

	return cursor
}

// renderConfigurationView renders the full-width configuration view
func (m Model) renderConfigurationView() string {
	return m.renderConfigurationViewWithWidth(m.width)
}

// renderConfigurationViewWithWidth renders the configuration view with specified width
func (m Model) renderConfigurationViewWithWidth(width int) string {
	var content []string

	// Title (anchored)
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Align(lipgloss.Center).
		Width(width - 2)

	content = append(content, titleStyle.Render("Configuration Settings"))
	content = append(content, "")

	// Anchored fields (always visible at top)
	anchoredFields := []struct {
		field Field
		label string
		value string
		help  string
	}{
		{ChatModelField, "Chat Model", m.editConfig.ChatModel, "Model used for chat conversations (Enter: Select from list)"},
	}

	// Render anchored fields
	for _, field := range anchoredFields {
		content = append(content, m.renderField(field.field, field.label, field.value, field.help))
		content = append(content, "")
	}

	// System Prompt field (no longer expands in place)
	content = append(content, m.renderField(DefaultSystemPromptField, "Default System Prompt", m.truncateSystemPrompt(), "System prompt sent with each message (Enter: View/Edit in panel)"))
	content = append(content, "")

	// Remaining fields (pushed down by system prompt expansion)
	remainingFields := []struct {
		field Field
		label string
		value string
		help  string
	}{
		{OllamaURLField, "Ollama URL", m.editConfig.OllamaURL, "URL of the Ollama server"},
		{LogLevelField, "Log Level", m.editConfig.LogLevel, "Logging level (Enter/Space: Cycle through debug → info → warn → error)"},
		{EnableFileLoggingField, "Enable File Logging", fmt.Sprintf("%t", m.editConfig.EnableFileLogging), "Enable logging to file (Enter/Space: Toggle)"},
		{AgentsFileEnabledField, "AGENTS.md Detection", fmt.Sprintf("%t", m.editConfig.AgentsFileEnabled), "Automatically detect and use AGENTS.md files from working directory (Enter/Space: Toggle)"},
	}

	// Render remaining fields
	for _, field := range remainingFields {
		content = append(content, m.renderField(field.field, field.label, field.value, field.help))
		content = append(content, "")
	}

	// Help text (no extra empty line since we already have one from the last field)
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(width - 2)
	content = append(content, helpStyle.Render("↑/↓: Navigate • Enter: Edit/Select (auto-saves when complete) • S: Manual Save • R: Reset to defaults"))
	if m.editing {
		content = append(content, helpStyle.Render("Enter: Save • Esc: Cancel"))
	}

	if m.showModelPanel {
		content = append(content, helpStyle.Render("Model Selection: ↑/↓: Navigate • Enter: Select • Esc: Cancel"))
	}

	if m.showSystemPromptPanel {
		if m.systemPromptEditMode {
			content = append(content, helpStyle.Render("System Prompt Editor: Ctrl+S: Save • Esc: Close"))
		} else {
			content = append(content, helpStyle.Render("System Prompt Viewer: Ctrl+E: Edit • Esc: Close"))
		}
	}
	// Message
	if m.message != "" {
		content = append(content, "")
		content = append(content, m.messageStyle.Render(m.message))
	}

	// The height we receive is already the content height (tab bar and footer already subtracted by main TUI)
	// Add a small adjustment to fill remaining space completely
	contentHeight := m.height + 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Join all content without pre-trimming - let lipgloss handle the height constraint
	fullContent := strings.Join(content, "\n")

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8A7FD8")).
		Padding(1, 2).
		Width(width - 2).
		Height(contentHeight) // Set explicit height to fill remaining space

	return containerStyle.Render(fullContent)
}

// truncateSystemPrompt returns a truncated version of the system prompt
func (m Model) truncateSystemPrompt() string {
	systemPrompt := m.editConfig.DefaultSystemPrompt

	// Always truncate to a reasonable length for display
	maxLength := 100
	if len(systemPrompt) <= maxLength {
		return systemPrompt
	}

	// Find a good truncation point (try to break at word boundary)
	truncated := systemPrompt[:maxLength]
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > maxLength-20 {
		truncated = systemPrompt[:lastSpace]
	}

	return truncated + "..."
}

// renderField renders a configuration field
func (m Model) renderField(field Field, label, value, help string) string {
	isActive := field == m.activeField
	isEditing := m.editing && isActive

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15"))
	if isActive {
		labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)
		valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62"))
	}

	// Display value with cursor if editing
	displayValue := value
	if isEditing {
		displayValue = m.input
		if m.cursor <= len(displayValue) {
			if m.cursor == len(displayValue) {
				displayValue += "|"
			} else {
				displayValue = displayValue[:m.cursor] + "|" + displayValue[m.cursor+1:]
			}
		}
	} else if field == EnableFileLoggingField || field == AgentsFileEnabledField {
		// Special formatting for toggle fields
		var toggleSymbol, toggleColor string
		if value == "true" {
			toggleSymbol = "●"
			toggleColor = "10" // Green
		} else {
			toggleSymbol = "○"
			toggleColor = "240" // Gray
		}
		toggleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(toggleColor))
		displayValue = fmt.Sprintf("%s %s", toggleStyle.Render(toggleSymbol), value)
	}

	// Add connectivity status for URL fields
	var statusIndicator string
	switch field {
	case OllamaURLField:
		statusIndicator = m.formatInlineConnectionStatus(m.ollamaStatus)
	}

	// Format the field with status indicator if applicable
	fieldLine := fmt.Sprintf("%s: %s", labelStyle.Render(label), valueStyle.Render(displayValue))
	if statusIndicator != "" {
		fieldLine = fmt.Sprintf("%s: %s %s", labelStyle.Render(label), valueStyle.Render(displayValue), statusIndicator)
	}

	if isActive {
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
		return fieldLine + "\n" + helpStyle.Render("  "+help)
	}

	return fieldLine
}

// wrapText wraps text to fit within the specified width with intelligent word breaking
func (m Model) wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	lines := strings.Split(text, "\n")
	var wrappedLines []string

	for _, line := range lines {
		if line == "" {
			wrappedLines = append(wrappedLines, "")
			continue
		}

		runes := []rune(line)
		if len(runes) <= width {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		// Word-aware wrapping
		words := strings.Fields(line)
		if len(words) == 0 {
			// If no words (just whitespace), handle as before
			for len(runes) > 0 {
				endIdx := width
				if endIdx > len(runes) {
					endIdx = len(runes)
				}
				wrappedLines = append(wrappedLines, string(runes[:endIdx]))
				runes = runes[endIdx:]
			}
			continue
		}

		var currentLine strings.Builder
		for _, word := range words {
			wordRunes := []rune(word)

			// If the word itself is longer than the width, we need to break it
			if len(wordRunes) > width {
				// First, add current line if it has content
				if currentLine.Len() > 0 {
					wrappedLines = append(wrappedLines, currentLine.String())
					currentLine.Reset()
				}

				// Break the long word across multiple lines
				for len(wordRunes) > 0 {
					endIdx := width
					if endIdx > len(wordRunes) {
						endIdx = len(wordRunes)
					}
					wrappedLines = append(wrappedLines, string(wordRunes[:endIdx]))
					wordRunes = wordRunes[endIdx:]
				}
				continue
			}

			// Check if adding this word would exceed the width
			spaceNeeded := 0
			if currentLine.Len() > 0 {
				spaceNeeded = 1 // for the space before the word
			}

			if currentLine.Len() > 0 && len([]rune(currentLine.String()))+spaceNeeded+len(wordRunes) > width {
				// Start a new line
				wrappedLines = append(wrappedLines, currentLine.String())
				currentLine.Reset()
			}

			// Add word to current line
			if currentLine.Len() > 0 {
				currentLine.WriteString(" ")
			}
			currentLine.WriteString(word)
		}

		// Add the last line if it has content
		if currentLine.Len() > 0 {
			wrappedLines = append(wrappedLines, currentLine.String())
		}
	}

	return wrappedLines
}

// formatInlineConnectionStatus formats a connection status for inline display
func (m Model) formatInlineConnectionStatus(status connection.Status) string {
	var statusText, statusColor string

	switch status {
	case connection.StatusConnected:
		statusText = "✓"
		statusColor = "10" // Green
	case connection.StatusDisconnected:
		statusText = "✗"
		statusColor = "9" // Red
	case connection.StatusChecking:
		statusText = "⟳"
		statusColor = "11" // Yellow
	default:
		statusText = "?"
		statusColor = "240" // Gray
	}

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
	return statusStyle.Render(statusText)
}

// getCurrentFieldValue gets the current value of the active field
func (m Model) getCurrentFieldValue() string {
	switch m.activeField {
	case ChatModelField:
		return m.editConfig.ChatModel
	case OllamaURLField:
		return m.editConfig.OllamaURL
	case DefaultSystemPromptField:
		return m.editConfig.DefaultSystemPrompt
	case LogLevelField:
		return m.editConfig.LogLevel
	case EnableFileLoggingField:
		return fmt.Sprintf("%t", m.editConfig.EnableFileLogging)
	case AgentsFileEnabledField:
		return fmt.Sprintf("%t", m.editConfig.AgentsFileEnabled)
	default:
		return ""
	}
}

// getFieldName returns a human-readable field name for logging
func (m Model) getFieldName(field Field) string {
	switch field {
	case ChatModelField:
		return "chat_model"
	case OllamaURLField:
		return "ollama_url"
	case DefaultSystemPromptField:
		return "default_system_prompt"
	case LogLevelField:
		return "log_level"
	case EnableFileLoggingField:
		return "enable_file_logging"
	case AgentsFileEnabledField:
		return "agents_file_enabled"
	default:
		return "unknown_field"
	}
}

// setCurrentFieldValue sets the current value of the active field
func (m Model) setCurrentFieldValue(value string) error {
	logger := logging.WithComponent("configuration_tab")

	switch m.activeField {
	case ChatModelField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("chat model cannot be empty")
		}
		oldValue := m.editConfig.ChatModel
		m.editConfig.ChatModel = strings.TrimSpace(value)
		logger.Info("Chat model changed", "old_value", oldValue, "new_value", m.editConfig.ChatModel)

	case OllamaURLField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("ollama URL cannot be empty")
		}
		oldValue := m.editConfig.OllamaURL
		m.editConfig.OllamaURL = strings.TrimSpace(value)
		logger.Info("Ollama URL changed", "old_value", oldValue, "new_value", m.editConfig.OllamaURL)

	case DefaultSystemPromptField:
		// Allow empty system prompt, but trim whitespace
		oldValue := m.editConfig.DefaultSystemPrompt
		newValue := strings.TrimSpace(value)
		m.editConfig.DefaultSystemPrompt = newValue

		// Save system prompt to file using the new method
		if err := m.config.SetSystemPrompt(newValue); err != nil {
			logger.Error("Failed to save system prompt to file", "error", err)
			// Don't fail the field setting, but log the error
			// The value will still be saved to the editConfig for UI purposes
		}

		logger.Info("Default system prompt changed", "old_length", len(oldValue), "new_length", len(newValue))

	case LogLevelField:
		// LogLevel is now handled by cycling, but keep validation for programmatic use
		value = strings.ToLower(strings.TrimSpace(value))
		validLevels := []string{"debug", "info", "warn", "error"}
		isValid := false
		for _, level := range validLevels {
			if value == level {
				isValid = true
				break
			}
		}
		if !isValid {
			logger.Error("Invalid log level", "value", value, "valid_levels", validLevels)
			return fmt.Errorf("log level must be one of: debug, info, warn, error")
		}
		oldValue := m.editConfig.LogLevel
		m.editConfig.LogLevel = value
		logger.Info("Log level changed", "old_value", oldValue, "new_value", m.editConfig.LogLevel)

	case EnableFileLoggingField:
		value = strings.ToLower(strings.TrimSpace(value))
		var fileLogging bool
		if value == "true" || value == "t" || value == "yes" || value == "y" || value == "1" {
			fileLogging = true
		} else if value == "false" || value == "f" || value == "no" || value == "n" || value == "0" {
			fileLogging = false
		} else {
			logger.Error("Invalid file logging value", "value", value)
			return fmt.Errorf("enable file logging must be true or false")
		}
		oldValue := m.editConfig.EnableFileLogging
		m.editConfig.EnableFileLogging = fileLogging
		logger.Info("File logging enabled changed", "old_value", oldValue, "new_value", m.editConfig.EnableFileLogging)

	case AgentsFileEnabledField:
		value = strings.ToLower(strings.TrimSpace(value))
		var agentsEnabled bool
		if value == "true" || value == "t" || value == "yes" || value == "y" || value == "1" {
			agentsEnabled = true
		} else if value == "false" || value == "f" || value == "no" || value == "n" || value == "0" {
			agentsEnabled = false
		} else {
			logger.Error("Invalid agents file enabled value", "value", value)
			return fmt.Errorf("agents file enabled must be true or false")
		}
		oldValue := m.editConfig.AgentsFileEnabled
		m.editConfig.AgentsFileEnabled = agentsEnabled
		logger.Info("Agents file detection changed", "old_value", oldValue, "new_value", m.editConfig.AgentsFileEnabled)
	}

	return nil
}

// syncEditConfigWithMain synchronizes editConfig with the current main config
// This ensures we don't lose any changes made by other tabs (like MCP servers)
func (m *Model) syncEditConfigWithMain() {
	// Preserve the current edit values for fields that might be different
	editValues := struct {
		chatModel           string
		ollamaURL           string
		defaultSystemPrompt string
		logLevel            string
		enableFileLogging   bool
		agentsFileEnabled   bool
	}{
		chatModel:           m.editConfig.ChatModel,
		ollamaURL:           m.editConfig.OllamaURL,
		defaultSystemPrompt: m.editConfig.DefaultSystemPrompt,
		logLevel:            m.editConfig.LogLevel,
		enableFileLogging:   m.editConfig.EnableFileLogging,
		agentsFileEnabled:   m.editConfig.AgentsFileEnabled,
	}

	// Start with the current main config to preserve all non-edited fields
	*m.editConfig = *m.config

	// Restore the edited values
	m.editConfig.ChatModel = editValues.chatModel
	m.editConfig.OllamaURL = editValues.ollamaURL
	m.editConfig.DefaultSystemPrompt = editValues.defaultSystemPrompt
	m.editConfig.LogLevel = editValues.logLevel
	m.editConfig.EnableFileLogging = editValues.enableFileLogging
	m.editConfig.AgentsFileEnabled = editValues.agentsFileEnabled
}

// saveConfiguration saves the configuration to disk
func (m Model) saveConfiguration() (tea.Model, tea.Cmd) {
	logger := logging.WithComponent("configuration_tab")
	logger.Info("Attempting to save configuration")

	// Sync editConfig with main config to preserve any changes made by other tabs
	m.syncEditConfigWithMain()

	if err := m.editConfig.Validate(); err != nil {
		logger.Error("Configuration validation failed", "error", err)
		m.message = fmt.Sprintf("Validation error: %s", err.Error())
		m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("9"))
		return m, nil
	}

	// Check if logging-related settings changed
	logSettingsChanged := m.config.LogLevel != m.editConfig.LogLevel || m.config.EnableFileLogging != m.editConfig.EnableFileLogging

	if err := m.editConfig.Save(); err != nil {
		logger.Error("Configuration save failed", "error", err)
		m.message = fmt.Sprintf("Save error: %s", err.Error())
		m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("9"))
		return m, nil
	}

	// Update the main config
	*m.config = *m.editConfig
	logger.Info("Configuration saved successfully")

	// If logging settings changed, reconfigure the logging system
	if logSettingsChanged {
		logConfig := &logging.Config{
			Level:        logging.LogLevel(m.config.GetLogLevel()),
			EnableFile:   m.config.EnableFileLogging,
			LogDir:       logging.DefaultDir(),
			EnableStderr: false, // Keep stderr disabled for TUI mode
		}

		if err := logging.Reconfigure(logConfig); err != nil {
			logger.Error("Failed to reconfigure logging system", "error", err)
			m.message = fmt.Sprintf("Configuration saved but logging reconfiguration failed: %s", err.Error())
			m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for warning
		} else {
			logger.Info("Logging system reconfigured", "new_level", m.config.LogLevel, "file_logging", m.config.EnableFileLogging)
			m.message = "Configuration saved and logging reconfigured successfully!"
			m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))
		}
	} else {
		m.message = "Configuration saved successfully!"
		m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))
	}

	// Send configuration update message to notify other tabs
	configUpdateCmd := func() tea.Msg {
		return ConfigUpdatedMsg{Config: m.config}
	}

	return m, configUpdateCmd
}

// autoSaveConfiguration automatically saves the configuration to disk after validation
// Returns a command to send the config update message and an error if save fails
func (m *Model) autoSaveConfiguration() (tea.Cmd, error) {
	logger := logging.WithComponent("configuration_tab")
	logger.Debug("Auto-saving configuration")

	// Sync editConfig with main config to preserve any changes made by other tabs
	m.syncEditConfigWithMain()

	if err := m.editConfig.Validate(); err != nil {
		logger.Error("Configuration validation failed during auto-save", "error", err)
		return nil, err
	}

	// Check if logging-related settings changed
	logSettingsChanged := m.config.LogLevel != m.editConfig.LogLevel || m.config.EnableFileLogging != m.editConfig.EnableFileLogging

	if err := m.editConfig.Save(); err != nil {
		logger.Error("Configuration auto-save failed", "error", err)
		return nil, err
	}

	// Update the main config
	*m.config = *m.editConfig
	logger.Debug("Configuration auto-saved successfully")

	// If logging settings changed, reconfigure the logging system
	if logSettingsChanged {
		logConfig := &logging.Config{
			Level:        logging.LogLevel(m.config.GetLogLevel()),
			EnableFile:   m.config.EnableFileLogging,
			LogDir:       logging.DefaultDir(),
			EnableStderr: false, // Keep stderr disabled for TUI mode
		}

		if err := logging.Reconfigure(logConfig); err != nil {
			logger.Error("Failed to reconfigure logging system", "error", err)
			// Don't fail the save, but log the error
		} else {
			logger.Info("Logging system reconfigured", "new_level", m.config.LogLevel, "file_logging", m.config.EnableFileLogging)
		}
	}

	// Create command to send configuration update message
	configUpdateCmd := func() tea.Msg {
		return ConfigUpdatedMsg{Config: m.config}
	}

	return configUpdateCmd, nil
}

// ensureCursorVisible adjusts scroll position to keep cursor visible
func (m *Model) ensureCursorVisible() {
	if !m.systemPromptEditMode {
		return
	}

	logger := logging.WithComponent("system_prompt_cursor")

	// Use the original text with indicators for line calculation
	originalText := m.systemPromptEditInput
	displayTextWithIndicators := m.renderVisibleChars(originalText)
	textWidth := (m.width / 3) - 6 // Account for panel width and padding
	if textWidth < 20 {
		textWidth = 20
	}

	lines := m.wrapText(displayTextWithIndicators, textWidth)

	// Find which line the cursor is on by counting characters in original text
	charCount := 0
	cursorLine := 0

	// Convert cursor position in original text to display text position
	displayCursor := m.convertCursorToDisplayPosition(originalText, m.systemPromptEditCursor)

	for i, line := range lines {
		lineLength := len([]rune(line))
		if charCount+lineLength >= displayCursor {
			cursorLine = i
			break
		}
		charCount += lineLength
		// Account for newline character if not at end
		if i < len(lines)-1 {
			charCount++
		}
	}

	// Calculate available height for content
	textAreaHeight := m.height - 12
	if textAreaHeight < 5 {
		textAreaHeight = 5
	}

	// Calculate max scroll to ensure we can see the end of text
	maxScroll := len(lines) - textAreaHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	oldScrollY := m.systemPromptScrollY

	// Adjust scroll to keep cursor visible
	if cursorLine < m.systemPromptScrollY {
		// Cursor is above visible area - scroll up
		m.systemPromptScrollY = cursorLine
	} else if cursorLine >= m.systemPromptScrollY+textAreaHeight {
		// Cursor is below visible area - scroll down
		m.systemPromptScrollY = cursorLine - textAreaHeight + 1
	}

	// Ensure scroll doesn't go negative or exceed maximum
	if m.systemPromptScrollY < 0 {
		m.systemPromptScrollY = 0
	}
	if m.systemPromptScrollY > maxScroll {
		m.systemPromptScrollY = maxScroll
	}

	logger.Debug("Cursor visibility adjusted",
		"cursor_pos", m.systemPromptEditCursor,
		"cursor_line", cursorLine,
		"old_scroll", oldScrollY,
		"new_scroll", m.systemPromptScrollY,
		"max_scroll", maxScroll,
		"total_lines", len(lines),
		"text_area_height", textAreaHeight)
} // convertCursorToDisplayPosition converts cursor position from original text to display text
func (m Model) convertCursorToDisplayPosition(originalText string, cursorPos int) int {
	if cursorPos <= 0 {
		return 0
	}

	originalRunes := []rune(originalText)
	if cursorPos >= len(originalRunes) {
		return len([]rune(m.renderVisibleChars(originalText)))
	}

	// Convert the text up to cursor position
	textBeforeCursor := string(originalRunes[:cursorPos])
	displayTextBeforeCursor := m.renderVisibleChars(textBeforeCursor)
	return len([]rune(displayTextBeforeCursor))
}

// renderVisibleChars adds visual indicators for hidden characters while preserving function
func (m Model) renderVisibleChars(text string) string {
	result := strings.ReplaceAll(text, "\t", "→\t") // Show → before actual tabs
	return result
}

// applyMarkdownStyling adds basic markdown syntax highlighting using lipgloss colors
func (m Model) applyMarkdownStyling(text string) string {
	lines := strings.Split(text, "\n")
	styledLines := make([]string, len(lines))

	for i, line := range lines {
		styled := line

		// Headers
		if strings.HasPrefix(line, "# ") {
			styled = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Render(line)
		} else if strings.HasPrefix(line, "## ") {
			styled = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(line)
		} else if strings.HasPrefix(line, "### ") {
			styled = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render(line)
		} else if strings.HasPrefix(line, "#### ") {
			styled = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(line)
		} else {
			// Bold text **text**
			if strings.Contains(line, "**") {
				// Simple bold highlighting - this is basic, could be improved
				styled = strings.ReplaceAll(styled, "**", lipgloss.NewStyle().Bold(true).Render("**"))
			}

			// Italic text *text*
			if strings.Contains(line, "*") && !strings.Contains(line, "**") {
				styled = strings.ReplaceAll(styled, "*", lipgloss.NewStyle().Italic(true).Render("*"))
			}

			// Code blocks ```
			if strings.HasPrefix(line, "```") {
				styled = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(line)
			}

			// Inline code `code`
			if strings.Contains(line, "`") {
				styled = strings.ReplaceAll(styled, "`", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("`"))
			}

			// Lists
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
				styled = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("• ") + line[2:]
			}

			// Numbered lists (basic detection)
			if len(line) > 0 && line[0] >= '0' && line[0] <= '9' && strings.Contains(line, ". ") {
				styled = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(line)
			}
		}

		styledLines[i] = styled
	}

	return strings.Join(styledLines, "\n")
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IsInSystemPromptEditMode returns true if the system prompt panel is visible and in edit mode
func (m Model) IsInSystemPromptEditMode() bool {
	return m.showSystemPromptPanel && m.systemPromptEditMode
}
