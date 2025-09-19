package configuration

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevensen/gollama-chat/internal/configuration"
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
	DarkModeField
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
	// Create a copy for editing
	editConfig := &configuration.Config{
		ChatModel:           config.ChatModel,
		EmbeddingModel:      config.EmbeddingModel,
		RAGEnabled:          config.RAGEnabled,
		OllamaURL:           config.OllamaURL,
		ChromaDBURL:         config.ChromaDBURL,
		ChromaDBDistance:    config.ChromaDBDistance,
		MaxDocuments:        config.MaxDocuments,
		DarkMode:            config.DarkMode,
		SelectedCollections: make(map[string]bool),
		DefaultSystemPrompt: config.DefaultSystemPrompt,
	}

	// Copy the selectedCollections map
	maps.Copy(editConfig.SelectedCollections, config.SelectedCollections)

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
		switch msg.Server {
		case "ollama":
			m.ollamaStatus = msg.Status
		case "chromadb":
			m.chromaDBStatus = msg.Status
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
		if saveErr, updateCmd := m.autoSaveConfiguration(); saveErr != nil {
			m.message = fmt.Sprintf("Selected: %s (save failed: %s)", msg.ModelName, saveErr.Error())
			m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for warning
			return m, nil
		} else {
			m.message = fmt.Sprintf("Selected and saved: %s", msg.ModelName)
			m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))
			return m, updateCmd
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
		if m.activeField < DarkModeField { // Updated to use actual last field
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
		} else {
			// Start editing the active field
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
		if saveErr, updateCmd := m.autoSaveConfiguration(); saveErr != nil {
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
			// Auto-save the configuration after successful field update
			saveErr, updateCmd := m.autoSaveConfiguration()
			if saveErr != nil {
				m.message = fmt.Sprintf("Field updated but save failed: %s", saveErr.Error())
				m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("11")) // Yellow for warning
				m.editing = false
				return m, nil
			} else {
				m.message = "Field updated and saved"
				m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))
			}

			// Check if we need to re-test connections
			var cmd tea.Cmd
			var cmds []tea.Cmd

			// Add the config update command
			cmds = append(cmds, updateCmd)

			switch m.activeField {
			case OllamaURLField:
				m.ollamaStatus = connection.StatusChecking
				cmd = connection.OllamaStatus(m.editConfig.OllamaURL)
			case ChromaDBURLField:
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
		{RAGEnabledField, "RAG Enabled", fmt.Sprintf("%t", m.editConfig.RAGEnabled), "Enable Retrieval Augmented Generation"},
		{OllamaURLField, "Ollama URL", m.editConfig.OllamaURL, "URL of the Ollama server"},
		{ChromaDBURLField, "ChromaDB URL", m.editConfig.ChromaDBURL, "URL of the ChromaDB server"},
		{ChromaDBDistanceField, "ChromaDB Distance", fmt.Sprintf("%.2f", m.editConfig.ChromaDBDistance), "Distance threshold for cosine similarity (0-2 range)"},
		{MaxDocumentsField, "Max Documents", fmt.Sprintf("%d", m.editConfig.MaxDocuments), "Maximum documents for RAG"},
		{DarkModeField, "Dark Mode", fmt.Sprintf("%t", m.editConfig.DarkMode), "Enable dark mode theme"},
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
	content = append(content, helpStyle.Render("↑/↓: Navigate • Enter: Edit/Select (auto-saves) • S: Save • R: Reset to defaults"))
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

	// Container
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8A7FD8")).
		Padding(1, 2).
		Width(width - 2).
		Height(m.height - 6)

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
	case DarkModeField:
		return fmt.Sprintf("%t", m.editConfig.DarkMode)
	case DefaultSystemPromptField:
		return m.editConfig.DefaultSystemPrompt
	default:
		return ""
	}
}

// setCurrentFieldValue sets the current value of the active field
func (m Model) setCurrentFieldValue(value string) error {
	switch m.activeField {
	case ChatModelField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Chat model cannot be empty")
		}
		m.editConfig.ChatModel = strings.TrimSpace(value)

	case EmbeddingModelField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Embedding model cannot be empty")
		}
		m.editConfig.EmbeddingModel = strings.TrimSpace(value)

	case RAGEnabledField:
		ragEnabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("RAG enabled must be true or false")
		}
		m.editConfig.RAGEnabled = ragEnabled

	case OllamaURLField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Ollama URL cannot be empty")
		}
		m.editConfig.OllamaURL = strings.TrimSpace(value)

	case ChromaDBURLField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("ChromaDB URL cannot be empty")
		}
		m.editConfig.ChromaDBURL = strings.TrimSpace(value)

	case ChromaDBDistanceField:
		distance, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return fmt.Errorf("ChromaDB distance must be a number")
		}
		if distance < 0 || distance > 1 {
			return fmt.Errorf("ChromaDB distance must be between 0.0 and 1.0")
		}
		m.editConfig.ChromaDBDistance = distance

	case MaxDocumentsField:
		maxDocs, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("Max documents must be a number")
		}
		if maxDocs <= 0 {
			return fmt.Errorf("Max documents must be greater than 0")
		}
		m.editConfig.MaxDocuments = maxDocs

	case DarkModeField:
		darkMode, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("Dark mode must be true or false")
		}
		m.editConfig.DarkMode = darkMode

	case DefaultSystemPromptField:
		// Allow empty system prompt, but trim whitespace
		m.editConfig.DefaultSystemPrompt = strings.TrimSpace(value)
	}

	return nil
}

// saveConfiguration saves the configuration to disk
func (m Model) saveConfiguration() (tea.Model, tea.Cmd) {
	if err := m.editConfig.Validate(); err != nil {
		m.message = fmt.Sprintf("Validation error: %s", err.Error())
		m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("9"))
		return m, nil
	}

	if err := m.editConfig.Save(); err != nil {
		m.message = fmt.Sprintf("Save error: %s", err.Error())
		m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("9"))
		return m, nil
	}

	// Update the main config
	*m.config = *m.editConfig

	m.message = "Configuration saved successfully!"
	m.messageStyle = m.messageStyle.Foreground(lipgloss.Color("10"))

	// Send configuration update message to notify other tabs
	configUpdateCmd := func() tea.Msg {
		return ConfigUpdatedMsg{Config: m.config}
	}

	return m, configUpdateCmd
}

// autoSaveConfiguration automatically saves the configuration to disk after validation
// Returns an error if save fails, nil if successful, and a command to send the config update message
func (m *Model) autoSaveConfiguration() (error, tea.Cmd) {
	if err := m.editConfig.Validate(); err != nil {
		return err, nil
	}

	if err := m.editConfig.Save(); err != nil {
		return err, nil
	}

	// Update the main config
	*m.config = *m.editConfig

	// Create command to send configuration update message
	configUpdateCmd := func() tea.Msg {
		return ConfigUpdatedMsg{Config: m.config}
	}

	return nil, configUpdateCmd
}
