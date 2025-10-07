package configuration

import (
	"fmt"
	"maps"
	"strconv"
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
	EmbeddingModelField
	DefaultSystemPromptField
	RAGEnabledField
	OllamaURLField
	ChromaDBURLField
	ChromaDBDistanceField
	MaxDocumentsField
	LogLevelField
	EnableFileLoggingField
)

// Model represents the configuration tab model
type Model struct {
	config         *configuration.Config
	editConfig     *configuration.Config // Copy for editing
	activeField    Field
	editing        bool
	input          string
	cursor         int
	width          int
	height         int
	message        string
	messageStyle   lipgloss.Style
	ollamaStatus   connection.Status
	chromaDBStatus connection.Status
	modelPanel     models.Model
	showModelPanel bool
}

// NewModel creates a new configuration model
func NewModel(config *configuration.Config) Model {
	logger := logging.WithComponent("configuration_tab")
	logger.Info("Creating new configuration model", "ollama_url", config.OllamaURL, "chromadb_url", config.ChromaDBURL, "rag_enabled", config.RAGEnabled)

	// Create a copy for editing
	editConfig := &configuration.Config{
		ChatModel:           config.ChatModel,
		EmbeddingModel:      config.EmbeddingModel,
		RAGEnabled:          config.RAGEnabled,
		OllamaURL:           config.OllamaURL,
		ChromaDBURL:         config.ChromaDBURL,
		ChromaDBDistance:    config.ChromaDBDistance,
		MaxDocuments:        config.MaxDocuments,
		SelectedCollections: make(map[string]bool),
		DefaultSystemPrompt: config.DefaultSystemPrompt,
		ToolTrustLevels:     make(map[string]int),
		MCPServers:          make([]configuration.MCPServer, len(config.MCPServers)),
		LogLevel:            config.LogLevel,
		EnableFileLogging:   config.EnableFileLogging,
	}

	// Copy the selectedCollections map
	maps.Copy(editConfig.SelectedCollections, config.SelectedCollections)

	// Copy the toolTrustLevels map
	maps.Copy(editConfig.ToolTrustLevels, config.ToolTrustLevels)

	// Copy the MCPServers slice
	copy(editConfig.MCPServers, config.MCPServers)

	return Model{
		config:         config,
		editConfig:     editConfig,
		activeField:    OllamaURLField,
		editing:        false,
		ollamaStatus:   connection.StatusUnknown,
		chromaDBStatus: connection.StatusUnknown,
		modelPanel:     models.NewModel(),
		showModelPanel: false,
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
		panelHeight := m.height - 10 // Leave room for other UI elements
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
				DefaultSystemPrompt: msg.Config.DefaultSystemPrompt,
				ToolTrustLevels:     make(map[string]int),
				MCPServers:          make([]configuration.MCPServer, len(msg.Config.MCPServers)),
				LogLevel:            msg.Config.LogLevel,
				EnableFileLogging:   msg.Config.EnableFileLogging,
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

	switch msg.String() {
	case "up", "k":
		if m.activeField > 0 {
			m.activeField--
		}

	case "down", "j":
		if m.activeField < EnableFileLoggingField { // Updated to use actual last field
			m.activeField++
		}

	case "enter", " ":
		// Check if we should show model selection panel
		if m.activeField == ChatModelField || m.activeField == EmbeddingModelField {
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
		} else if m.activeField == RAGEnabledField || m.activeField == EnableFileLoggingField {
			// Toggle boolean fields directly
			logger := logging.WithComponent("configuration_tab")
			fieldName := m.getFieldName(m.activeField)
			var oldValue, newValue bool

			switch m.activeField {
			case RAGEnabledField:
				oldValue = m.editConfig.RAGEnabled
				m.editConfig.RAGEnabled = !m.editConfig.RAGEnabled
				newValue = m.editConfig.RAGEnabled
			case EnableFileLoggingField:
				oldValue = m.editConfig.EnableFileLogging
				m.editConfig.EnableFileLogging = !m.editConfig.EnableFileLogging
				newValue = m.editConfig.EnableFileLogging
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
			if (m.activeField == OllamaURLField || m.activeField == ChromaDBURLField) && m.editConfig.ChatModel == "" {
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
			case ChromaDBURLField:
				logger.Info("Triggering ChromaDB connection test after URL change", "new_url", m.editConfig.ChromaDBURL)
				m.chromaDBStatus = connection.StatusChecking
				cmd = connection.ChromaDBStatus(m.editConfig.ChromaDBURL)
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

// renderConfigurationView renders the full-width configuration view
func (m Model) renderConfigurationView() string {
	return m.renderConfigurationViewWithWidth(m.width)
}

// renderConfigurationViewWithWidth renders the configuration view with specified width
func (m Model) renderConfigurationViewWithWidth(width int) string {
	var content []string

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Align(lipgloss.Center).
		Width(width - 2)

	content = append(content, titleStyle.Render("Configuration Settings"))
	content = append(content, "")

	// Configuration fields
	fields := []struct {
		field Field
		label string
		value string
		help  string
	}{
		{ChatModelField, "Chat Model", m.editConfig.ChatModel, "Model used for chat conversations (Enter: Select from list)"},
		{EmbeddingModelField, "Embedding Model", m.editConfig.EmbeddingModel, "Model for embeddings (Enter: Select from list)"},
		{DefaultSystemPromptField, "Default System Prompt", m.editConfig.DefaultSystemPrompt, "System prompt sent with each message"},
		{RAGEnabledField, "RAG Enabled", fmt.Sprintf("%t", m.editConfig.RAGEnabled), "Enable Retrieval Augmented Generation (Enter/Space: Toggle)"},
		{OllamaURLField, "Ollama URL", m.editConfig.OllamaURL, "URL of the Ollama server"},
		{ChromaDBURLField, "ChromaDB URL", m.editConfig.ChromaDBURL, "URL of the ChromaDB server"},
		{ChromaDBDistanceField, "ChromaDB Distance", fmt.Sprintf("%.2f", m.editConfig.ChromaDBDistance), "Distance threshold for cosine similarity (0-2 range)"},
		{MaxDocumentsField, "Max Documents", fmt.Sprintf("%d", m.editConfig.MaxDocuments), "Maximum documents for RAG"},
		{LogLevelField, "Log Level", m.editConfig.LogLevel, "Logging level (Enter/Space: Cycle through debug → info → warn → error)"},
		{EnableFileLoggingField, "Enable File Logging", fmt.Sprintf("%t", m.editConfig.EnableFileLogging), "Enable logging to file (Enter/Space: Toggle)"},
	}

	for _, field := range fields {
		content = append(content, m.renderField(field.field, field.label, field.value, field.help))
		content = append(content, "")
	}

	// Help text
	content = append(content, "")
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
	// Message
	if m.message != "" {
		content = append(content, "")
		content = append(content, m.messageStyle.Render(m.message))
	}

	// Container - calculate height like main TUI does for content area
	tabBarHeight := 1
	footerHeight := 1
	contentHeight := m.height - tabBarHeight - footerHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8A7FD8")).
		Padding(1, 2).
		Width(width - 2).
		Height(contentHeight) // Match main TUI's content height calculation

	return containerStyle.Render(strings.Join(content, "\n"))
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
				displayValue += "█"
			} else {
				displayValue = displayValue[:m.cursor] + "█" + displayValue[m.cursor+1:]
			}
		}
	} else if field == RAGEnabledField || field == EnableFileLoggingField {
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
	case ChromaDBURLField:
		statusIndicator = m.formatInlineConnectionStatus(m.chromaDBStatus)
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
	case EmbeddingModelField:
		return m.editConfig.EmbeddingModel
	case RAGEnabledField:
		return fmt.Sprintf("%t", m.editConfig.RAGEnabled)
	case OllamaURLField:
		return m.editConfig.OllamaURL
	case ChromaDBURLField:
		return m.editConfig.ChromaDBURL
	case ChromaDBDistanceField:
		return fmt.Sprintf("%.2f", m.editConfig.ChromaDBDistance)
	case MaxDocumentsField:
		return fmt.Sprintf("%d", m.editConfig.MaxDocuments)
	case DefaultSystemPromptField:
		return m.editConfig.DefaultSystemPrompt
	case LogLevelField:
		return m.editConfig.LogLevel
	case EnableFileLoggingField:
		return fmt.Sprintf("%t", m.editConfig.EnableFileLogging)
	default:
		return ""
	}
}

// getFieldName returns a human-readable field name for logging
func (m Model) getFieldName(field Field) string {
	switch field {
	case ChatModelField:
		return "chat_model"
	case EmbeddingModelField:
		return "embedding_model"
	case RAGEnabledField:
		return "rag_enabled"
	case OllamaURLField:
		return "ollama_url"
	case ChromaDBURLField:
		return "chromadb_url"
	case ChromaDBDistanceField:
		return "chromadb_distance"
	case MaxDocumentsField:
		return "max_documents"
	case DefaultSystemPromptField:
		return "default_system_prompt"
	case LogLevelField:
		return "log_level"
	case EnableFileLoggingField:
		return "enable_file_logging"
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

	case EmbeddingModelField:
		// Allow empty embedding model when RAG is disabled
		oldValue := m.editConfig.EmbeddingModel
		m.editConfig.EmbeddingModel = strings.TrimSpace(value)
		logger.Info("Embedding model changed", "old_value", oldValue, "new_value", m.editConfig.EmbeddingModel)

	case RAGEnabledField:
		ragEnabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			logger.Error("Invalid RAG enabled value", "value", value, "error", err)
			return fmt.Errorf("RAG enabled must be true or false")
		}
		oldValue := m.editConfig.RAGEnabled
		m.editConfig.RAGEnabled = ragEnabled
		logger.Info("RAG enabled changed", "old_value", oldValue, "new_value", m.editConfig.RAGEnabled)

	case OllamaURLField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("ollama URL cannot be empty")
		}
		oldValue := m.editConfig.OllamaURL
		m.editConfig.OllamaURL = strings.TrimSpace(value)
		logger.Info("Ollama URL changed", "old_value", oldValue, "new_value", m.editConfig.OllamaURL)

	case ChromaDBURLField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("ChromaDB URL cannot be empty")
		}
		oldValue := m.editConfig.ChromaDBURL
		m.editConfig.ChromaDBURL = strings.TrimSpace(value)
		logger.Info("ChromaDB URL changed", "old_value", oldValue, "new_value", m.editConfig.ChromaDBURL)

	case ChromaDBDistanceField:
		distance, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			logger.Error("Invalid ChromaDB distance value", "value", value, "error", err)
			return fmt.Errorf("ChromaDB distance must be a number")
		}
		if distance < 0 || distance > 2 {
			logger.Error("ChromaDB distance out of range", "value", distance)
			return fmt.Errorf("ChromaDB distance must be between 0.0 and 2.0")
		}
		oldValue := m.editConfig.ChromaDBDistance
		m.editConfig.ChromaDBDistance = distance
		logger.Info("ChromaDB distance changed", "old_value", oldValue, "new_value", m.editConfig.ChromaDBDistance)

	case MaxDocumentsField:
		maxDocs, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			logger.Error("Invalid max documents value", "value", value, "error", err)
			return fmt.Errorf("max documents must be a number")
		}
		// Allow 0 max documents when RAG is disabled
		if maxDocs < 0 {
			logger.Error("Max documents cannot be negative", "value", maxDocs)
			return fmt.Errorf("max documents must be 0 or greater")
		}
		oldValue := m.editConfig.MaxDocuments
		m.editConfig.MaxDocuments = maxDocs
		logger.Info("Max documents changed", "old_value", oldValue, "new_value", m.editConfig.MaxDocuments)

	case DefaultSystemPromptField:
		// Allow empty system prompt, but trim whitespace
		oldValue := m.editConfig.DefaultSystemPrompt
		m.editConfig.DefaultSystemPrompt = strings.TrimSpace(value)
		logger.Info("Default system prompt changed", "old_length", len(oldValue), "new_length", len(m.editConfig.DefaultSystemPrompt))

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
	}

	return nil
}

// syncEditConfigWithMain synchronizes editConfig with the current main config
// This ensures we don't lose any changes made by other tabs (like MCP servers)
func (m *Model) syncEditConfigWithMain() {
	// Preserve the current edit values for fields that might be different
	editValues := struct {
		chatModel           string
		embeddingModel      string
		ragEnabled          bool
		ollamaURL           string
		chromaDBURL         string
		chromaDBDistance    float64
		maxDocuments        int
		defaultSystemPrompt string
		logLevel            string
		enableFileLogging   bool
	}{
		chatModel:           m.editConfig.ChatModel,
		embeddingModel:      m.editConfig.EmbeddingModel,
		ragEnabled:          m.editConfig.RAGEnabled,
		ollamaURL:           m.editConfig.OllamaURL,
		chromaDBURL:         m.editConfig.ChromaDBURL,
		chromaDBDistance:    m.editConfig.ChromaDBDistance,
		maxDocuments:        m.editConfig.MaxDocuments,
		defaultSystemPrompt: m.editConfig.DefaultSystemPrompt,
		logLevel:            m.editConfig.LogLevel,
		enableFileLogging:   m.editConfig.EnableFileLogging,
	}

	// Start with the current main config to preserve all non-edited fields
	*m.editConfig = *m.config

	// Restore the edited values
	m.editConfig.ChatModel = editValues.chatModel
	m.editConfig.EmbeddingModel = editValues.embeddingModel
	m.editConfig.RAGEnabled = editValues.ragEnabled
	m.editConfig.OllamaURL = editValues.ollamaURL
	m.editConfig.ChromaDBURL = editValues.chromaDBURL
	m.editConfig.ChromaDBDistance = editValues.chromaDBDistance
	m.editConfig.MaxDocuments = editValues.maxDocuments
	m.editConfig.DefaultSystemPrompt = editValues.defaultSystemPrompt
	m.editConfig.LogLevel = editValues.logLevel
	m.editConfig.EnableFileLogging = editValues.enableFileLogging
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

	if err := m.editConfig.Save(); err != nil {
		logger.Error("Configuration save failed", "error", err)
		m.message = fmt.Sprintf("Save error: %s", err.Error())
		m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("9"))
		return m, nil
	}

	// Update the main config
	*m.config = *m.editConfig
	logger.Info("Configuration saved successfully")

	m.message = "Configuration saved successfully!"
	m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))

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

	if err := m.editConfig.Save(); err != nil {
		logger.Error("Configuration auto-save failed", "error", err)
		return nil, err
	}

	// Update the main config
	*m.config = *m.editConfig
	logger.Debug("Configuration auto-saved successfully")

	// Create command to send configuration update message
	configUpdateCmd := func() tea.Msg {
		return ConfigUpdatedMsg{Config: m.config}
	}

	return configUpdateCmd, nil
}
