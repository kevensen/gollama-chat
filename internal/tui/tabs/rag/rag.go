package rag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
)

// Message types for async operations
type collectionsLoadedMsg struct {
	collections []Collection
	err         error
}

type connectionTestMsg struct {
	connected bool
	err       error
}

// ConfigUpdatedMsg is sent when the configuration has been updated
type ConfigUpdatedMsg struct {
	Config *configuration.Config
}

// CollectionsUpdatedMsg is sent when collections selection has changed
type CollectionsUpdatedMsg struct {
	SelectedCollections []string
}

// ConfigurationField represents fields that can be edited in the configuration section
type ConfigurationField int

const (
	RAGEnabledField ConfigurationField = iota
	EmbeddingModelField
	ChromaDBURLField
	ChromaDBDistanceField
	MaxDocumentsField
)

// ActivePane represents which pane is currently active
type ActivePane int

const (
	SettingsPane ActivePane = iota
	CollectionsPane
)

// Model represents the RAG collections tab model
type Model struct {
	config             *configuration.Config
	editConfig         *configuration.Config // Copy for editing configuration
	collectionsService *CollectionsService
	viewport           viewport.Model
	cursor             int
	collections        []Collection
	connected          bool
	loading            bool
	error              string
	width              int
	height             int
	ctx                context.Context
	
	// Two-pane edit mode state
	editMode           bool   // true when in edit mode (Ctrl+E), false in view mode
	activePane         ActivePane // which pane is currently active (settings or collections)
	activeConfigField  ConfigurationField
	editingConfig      bool   // true when editing a specific config field
	configInput        string // input text when editing config fields
	configCursor       int    // cursor position in config input
	configMessage      string // message for config operations
}

// NewModel creates a new RAG collections model
func NewModel(ctx context.Context, config *configuration.Config) Model {
	vp := viewport.New(0, 0)
	vp.YPosition = 0

	// Create a copy for editing configuration
	editConfig := &configuration.Config{
		ChatModel:        config.ChatModel,
		EmbeddingModel:   config.EmbeddingModel,
		RAGEnabled:       config.RAGEnabled,
		OllamaURL:        config.OllamaURL,
		ChromaDBURL:      config.ChromaDBURL,
		ChromaDBDistance: config.ChromaDBDistance,
		MaxDocuments:     config.MaxDocuments,
	}

	return Model{
		config:             config,
		editConfig:         editConfig,
		collectionsService: NewCollectionsService(config),
		viewport:           vp,
		cursor:             0,
		collections:        make([]Collection, 0),
		connected:          false,
		loading:            true,
		ctx:                ctx,
		editMode:           false, // Start in view mode
		activePane:         SettingsPane, // Start with settings pane
		activeConfigField:  RAGEnabledField,
		editingConfig:      false,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return m.testConnection()
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate viewport size (leave room for header and footer within the container)
		tabBarHeight := 1
		footerHeight := 1
		contentHeight := m.height - tabBarHeight - footerHeight
		if contentHeight < 1 {
			contentHeight = 1
		}

		headerHeight := 5     // Title + connection status + instructions
		ragFooterHeight := 3  // Selection count + controls
		containerPadding := 4 // Border (2) + padding (2)
		availableHeight := contentHeight - headerHeight - ragFooterHeight - containerPadding
		if availableHeight < 1 {
			availableHeight = 1
		}

		m.viewport.Width = int(float64(m.width) * 0.95)
		m.viewport.Height = availableHeight
		m.updateViewportContent()

	case ConfigUpdatedMsg:
		// Update configuration and refresh connection
		m.config = msg.Config
		m.collectionsService.Config = msg.Config
		m.loading = true
		m.connected = false
		m.error = ""
		m.updateViewportContent()
		// Test connection with new configuration
		return m, m.testConnection()

	case tea.KeyMsg:
		// Always allow these debug/control keys, even when loading
		switch msg.String() {
		case "c": // Show config (debug)
			m.error = fmt.Sprintf("Debug: ChromaDB URL = '%s', Loading = %t", m.config.ChromaDBURL, m.loading)
			m.updateViewportContent()
			return m, nil
		case "u": // Update/refresh config
			// Reload configuration from disk
			if newConfig, err := configuration.Load(); err == nil {
				m.config = newConfig
				m.collectionsService.Config = newConfig
				m.error = "Configuration refreshed"
				m.updateViewportContent()
			} else {
				m.error = fmt.Sprintf("Failed to reload config: %v", err)
				m.updateViewportContent()
			}
			return m, nil
		case "t": // Test connection
			m.loading = true
			m.error = ""
			return m, m.testConnection()
		case "x": // Force stop loading (emergency)
			m.loading = false
			m.error = "Loading cancelled by user"
			m.updateViewportContent()
			return m, nil
		case "ctrl+e": // Toggle edit mode
			m.editMode = !m.editMode
			m.editingConfig = false // Exit any field editing
			m.configMessage = ""
			return m, nil
		case "tab": // Switch between panes when in edit mode
			if m.editMode {
				if m.activePane == SettingsPane {
					m.activePane = CollectionsPane
				} else {
					m.activePane = SettingsPane
				}
				m.editingConfig = false // Exit any field editing when switching panes
			}
			return m, nil
		case "esc": // Exit edit mode or field editing
			if m.editingConfig {
				m.editingConfig = false
				m.configInput = ""
				m.configMessage = ""
			} else {
				m.editMode = false
			}
			return m, nil
		}

		// Handle configuration editing if in edit mode and editing a field
		if m.editMode && m.editingConfig {
			return m.handleConfigInput(msg)
		}

		// Only handle other keys when not loading
		if m.loading {
			return m, nil
		}

		// Handle keys based on edit mode and active pane
		if m.editMode {
			if m.activePane == SettingsPane {
				return m.handleConfigurationKeys(msg)
			} else {
				return m.handleCollectionsKeys(msg)
			}
		} else {
			// In view mode, only allow basic navigation for collections
			return m.handleViewModeKeys(msg)
		}

	case connectionTestMsg:
		logger := logging.WithComponent("rag-tab")
		logger.Info("Processing connection test message",
			"connected", msg.connected,
			"error", msg.err)

		m.loading = false
		m.connected = msg.connected
		if msg.err != nil {
			m.error = fmt.Sprintf("Connection failed: %v", msg.err)
			m.collections = make([]Collection, 0)
		} else {
			m.error = ""
			if m.connected {
				logger.Info("Connection successful, starting to load collections")
				return m, m.loadCollections(m.ctx)
			}
		}
		m.updateViewportContent()

	case collectionsLoadedMsg:
		logger := logging.WithComponent("rag-tab")
		logger.Info("Received collections loaded message",
			"error", msg.err,
			"collections_count", len(msg.collections))

		m.loading = false
		if msg.err != nil {
			m.error = fmt.Sprintf("Failed to load collections: %v", msg.err)
			m.collections = make([]Collection, 0)
		} else {
			m.error = ""
			m.collections = msg.collections
			// Reset cursor if it's out of bounds
			if m.cursor >= len(m.collections) {
				m.cursor = 0
			}
			// Update viewport content immediately after loading collections
			m.updateViewportContent()

			selectedCollections := m.collectionsService.GetSelectedCollections()
			logger.Info("Sending collections updated message",
				"selected_collections", selectedCollections,
				"count", len(selectedCollections))

			// Send message to notify about collection changes (all collections are selected by default)
			return m, tea.Cmd(func() tea.Msg {
				return CollectionsUpdatedMsg{
					SelectedCollections: selectedCollections,
				}
			})
		}
		m.updateViewportContent()
	}

	return m, nil
}

// View renders the model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Calculate available space for two-pane layout
	tabBarHeight := 1
	footerHeight := 1
	contentHeight := m.height - tabBarHeight - footerHeight + 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Title and mode indicator
	var output strings.Builder
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4")).
		PaddingBottom(1)
	output.WriteString(titleStyle.Render("RAG Configuration & Collections"))
	output.WriteString("\n")

	// Mode and instructions
	modeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	if m.editMode {
		activePaneName := "Settings"
		if m.activePane == CollectionsPane {
			activePaneName = "Collections"
		}
		output.WriteString(modeStyle.Render(fmt.Sprintf("Edit Mode - Active Pane: %s | Tab: Switch Panes | Esc: Exit Edit Mode", activePaneName)))
	} else {
		output.WriteString(modeStyle.Render("View Mode - Ctrl+E: Enter Edit Mode"))
	}
	output.WriteString("\n\n")

	// Two-pane layout
	leftPaneContent := m.renderSettingsPane()
	rightPaneContent := m.renderCollectionsPane()

	// Calculate pane widths (split roughly in half with some padding)
	paneWidth := (m.width - 6) / 2 // Account for borders and padding
	
	leftPaneStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8A7FD8")).
		Padding(1, 1).
		Width(paneWidth).
		Height(contentHeight - 8) // Account for title and instructions

	rightPaneStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8A7FD8")).
		Padding(1, 1).
		Width(paneWidth).
		Height(contentHeight - 8)

	// Highlight active pane when in edit mode
	if m.editMode {
		if m.activePane == SettingsPane {
			leftPaneStyle = leftPaneStyle.BorderForeground(lipgloss.Color("4"))
		} else {
			rightPaneStyle = rightPaneStyle.BorderForeground(lipgloss.Color("4"))
		}
	}

	leftPane := leftPaneStyle.Render(leftPaneContent)
	rightPane := rightPaneStyle.Render(rightPaneContent)

	// Combine panes side by side
	panesContent := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	output.WriteString(panesContent)

	return output.String()
}

// renderSettingsPane renders the RAG configuration settings pane
func (m Model) renderSettingsPane() string {
	var content strings.Builder

	// Pane title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4"))
	content.WriteString(titleStyle.Render("RAG Settings"))
	content.WriteString("\n\n")

	// Configuration fields
	fields := []struct {
		field ConfigurationField
		label string
		value string
		help  string
	}{
		{RAGEnabledField, "RAG Enabled", fmt.Sprintf("%t", m.editConfig.RAGEnabled), "Enable Retrieval Augmented Generation"},
		{EmbeddingModelField, "Embedding Model", m.editConfig.EmbeddingModel, "Model for embeddings"},
		{ChromaDBURLField, "ChromaDB URL", m.editConfig.ChromaDBURL, "URL of the ChromaDB server"},
		{ChromaDBDistanceField, "ChromaDB Distance", fmt.Sprintf("%.2f", m.editConfig.ChromaDBDistance), "Distance threshold for similarity"},
		{MaxDocumentsField, "Max Documents", fmt.Sprintf("%d", m.editConfig.MaxDocuments), "Maximum documents for RAG"},
	}

	for i, field := range fields {
		content.WriteString(m.renderConfigField(field.field, field.label, field.value, field.help))
		if i < len(fields)-1 {
			content.WriteString("\n")
		}
	}

	// Instructions based on mode
	if m.editMode && m.activePane == SettingsPane {
		content.WriteString("\n\n")
		instructionsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
		content.WriteString(instructionsStyle.Render("↑/↓: Navigate • Enter/Space: Edit/Toggle • S: Save"))

		if m.editingConfig {
			content.WriteString("\n")
			content.WriteString(instructionsStyle.Render("Enter: Save • Esc: Cancel"))
		}
	}

	// Message
	if m.configMessage != "" {
		messageStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
		content.WriteString("\n\n")
		content.WriteString(messageStyle.Render(m.configMessage))
	}

	return content.String()
}

// renderCollectionsPane renders the collections management pane
func (m Model) renderCollectionsPane() string {
	var content strings.Builder

	// Pane title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4"))
	content.WriteString(titleStyle.Render("Collections"))
	content.WriteString("\n\n")

	// Connection status
	connectionStyle := lipgloss.NewStyle()
	if m.connected {
		connectionStyle = connectionStyle.Foreground(lipgloss.Color("2")) // Green
		content.WriteString(connectionStyle.Render("✓ Connected to ChromaDB"))
	} else {
		connectionStyle = connectionStyle.Foreground(lipgloss.Color("1")) // Red
		content.WriteString(connectionStyle.Render("✗ Not connected to ChromaDB"))
		if m.error != "" {
			content.WriteString(fmt.Sprintf("\n%s", m.error))
		}
	}
	content.WriteString("\n\n")

	// Main content area
	if m.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("6"))
		content.WriteString(loadingStyle.Render("Loading..."))
	} else if !m.connected {
		content.WriteString("ChromaDB connection required.\nEnsure ChromaDB is running and\nconfiguration is correct.")
	} else if len(m.collections) == 0 {
		content.WriteString("No collections found in ChromaDB.")
	} else {
		// Limit viewport height for the pane
		content.WriteString(m.viewport.View())
	}

	// Instructions based on mode  
	if m.editMode && m.activePane == CollectionsPane && m.connected && !m.loading {
		content.WriteString("\n\n")
		instructionsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
		content.WriteString(instructionsStyle.Render("↑/↓: Navigate • Space: Toggle • Ctrl+A: Select All • Ctrl+D: Deselect All • R: Refresh"))
	} else if !m.editMode {
		content.WriteString("\n\n")
		instructionsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
		content.WriteString(instructionsStyle.Render("Ctrl+E: Enter Edit Mode"))
	}

	// Footer with selection count
	if m.connected && len(m.collections) > 0 {
		selectedCount := m.collectionsService.GetSelectedCount()
		totalCount := len(m.collections)

		countStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")).
			Bold(true)
		content.WriteString("\n\n")
		content.WriteString(countStyle.Render(fmt.Sprintf("Selected: %d/%d collections", selectedCount, totalCount)))
	}

	return content.String()
}

// renderConfigField renders a single configuration field
func (m Model) renderConfigField(field ConfigurationField, label, value, help string) string {
	var line strings.Builder

	isActive := field == m.activeConfigField && m.editMode && m.activePane == SettingsPane
	isEditing := m.editingConfig && isActive

	// Field label and value
	labelStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Left)

	valueStyle := lipgloss.NewStyle().
		Width(30).
		Align(lipgloss.Left)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Width(50)

	if isActive {
		if isEditing {
			// Show input field with cursor
			labelStyle = labelStyle.Foreground(lipgloss.Color("4")).Bold(true)
			
			// Show input with cursor
			inputText := m.configInput
			if m.configCursor < len(inputText) {
				inputText = inputText[:m.configCursor] + "█" + inputText[m.configCursor:]
			} else {
				inputText += "█"
			}
			valueStyle = valueStyle.Foreground(lipgloss.Color("2")).Bold(true)
			line.WriteString(labelStyle.Render(label) + " " + valueStyle.Render(inputText))
		} else {
			// Highlighted but not editing
			labelStyle = labelStyle.Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")).Bold(true)
			valueStyle = valueStyle.Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")).Bold(true)
			line.WriteString(labelStyle.Render(label) + " " + valueStyle.Render(value))
		}
	} else {
		// Normal display
		line.WriteString(labelStyle.Render(label) + " " + valueStyle.Render(value))
	}

	line.WriteString(" " + helpStyle.Render(help))
	return line.String()
}

// testConnection starts connection testing
func (m Model) testConnection() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		logger := logging.WithComponent("rag-tab")
		logger.Info("Testing connection to ChromaDB from RAG tab")

		err := m.collectionsService.TestConnection()
		connected := m.collectionsService.IsConnected()

		logger.Info("Connection test completed",
			"connected", connected,
			"error", err)

		return connectionTestMsg{
			connected: connected,
			err:       err,
		}
	})
}

// loadCollections starts collections loading
func (m Model) loadCollections(ctx context.Context) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		logger := logging.WithComponent("rag-tab")
		logger.Info("Starting to load collections from ChromaDB")

		err := m.collectionsService.LoadCollections(ctx)
		collections := m.collectionsService.GetCollections()

		logger.Info("Collections loading completed",
			"error", err,
			"collections_count", len(collections))

		return collectionsLoadedMsg{
			collections: collections,
			err:         err,
		}
	})
}

// updateViewportContent updates the viewport with current collections
func (m *Model) updateViewportContent() {
	if len(m.collections) == 0 {
		m.viewport.SetContent("")
		return
	}

	var content strings.Builder

	for i, collection := range m.collections {
		var line strings.Builder

		// Selection indicator
		if collection.Selected {
			line.WriteString("● ")
		} else {
			line.WriteString("○ ")
		}

		// Collection name
		nameStyle := lipgloss.NewStyle()
		if i == m.cursor {
			nameStyle = nameStyle.Background(lipgloss.Color("4")).
				Foreground(lipgloss.Color("15")).
				Bold(true)
		} else if collection.Selected {
			nameStyle = nameStyle.Foreground(lipgloss.Color("2")).
				Bold(true)
		}

		line.WriteString(nameStyle.Render(collection.Name))

		// Collection ID (truncated if too long)
		idStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

		if i == m.cursor {
			idStyle = idStyle.Background(lipgloss.Color("4")).
				Foreground(lipgloss.Color("7"))
		}

		collectionID := collection.ID
		if len(collectionID) > 20 {
			collectionID = collectionID[:17] + "..."
		}
		line.WriteString(idStyle.Render(fmt.Sprintf(" [%s]", collectionID)))

		// Add metadata count if available (for future expansion)
		if len(collection.Metadata) > 0 {
			metaStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))
			if i == m.cursor {
				metaStyle = metaStyle.Background(lipgloss.Color("4")).
					Foreground(lipgloss.Color("7"))
			}
			line.WriteString(metaStyle.Render(fmt.Sprintf(" (%d metadata)", len(collection.Metadata))))
		}

		content.WriteString(line.String())
		if i < len(m.collections)-1 {
			content.WriteString("\n")
		}
	}

	m.viewport.SetContent(content.String())
}

// updateViewportScroll updates viewport scroll position based on cursor
func (m *Model) updateViewportScroll() {
	if len(m.collections) == 0 {
		return
	}

	// Calculate which line the cursor is on
	lineHeight := 1
	cursorLine := m.cursor * lineHeight

	// Get viewport boundaries
	top := m.viewport.YOffset
	bottom := top + m.viewport.Height - 1

	// Scroll if cursor is outside viewport
	if cursorLine < top {
		m.viewport.YOffset = cursorLine
	} else if cursorLine > bottom {
		m.viewport.YOffset = cursorLine - m.viewport.Height + 1
	}
}

// GetSelectedCollections returns the list of selected collection names
func (m Model) GetSelectedCollections() []string {
	return m.collectionsService.GetSelectedCollections()
}

// handleConfigurationKeys handles key input when in configuration mode
func (m Model) handleConfigurationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.activeConfigField > RAGEnabledField {
			m.activeConfigField--
		}
		return m, nil
	case "down", "j":
		if m.activeConfigField < MaxDocumentsField {
			m.activeConfigField++
		}
		return m, nil
	case "enter", " ":
		switch m.activeConfigField {
		case RAGEnabledField:
			// Toggle RAG enabled
			m.editConfig.RAGEnabled = !m.editConfig.RAGEnabled
			
			// Auto-populate defaults when enabling RAG
			if m.editConfig.RAGEnabled {
				if m.editConfig.EmbeddingModel == "" {
					m.editConfig.EmbeddingModel = "nomic-embed-text:latest"
				}
				if m.editConfig.ChromaDBURL == "" {
					m.editConfig.ChromaDBURL = "http://localhost:8000"
				}
			}
			
			return m, m.saveConfiguration()
		default:
			// Start editing other fields
			m.editingConfig = true
			m.configInput = m.getCurrentConfigFieldValue()
			m.configCursor = len(m.configInput)
		}
		return m, nil
	case "s":
		// Manual save
		return m, m.saveConfiguration()
	}
	return m, nil
}

// handleViewModeKeys handles navigation keys when not in edit mode
func (m Model) handleViewModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		// Switch between panes
		if m.activePane == SettingsPane {
			m.activePane = CollectionsPane
		} else {
			m.activePane = SettingsPane
		}
		return m, nil
	}
	return m, nil
}

// handleCollectionsKeys handles key input when in collections mode  
func (m Model) handleCollectionsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.updateViewportScroll()
			m.updateViewportContent()
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.collections)-1 {
			m.cursor++
			m.updateViewportScroll()
			m.updateViewportContent()
		}
		return m, nil
	case " ", "enter": // Toggle selection
		if len(m.collections) > 0 && m.cursor < len(m.collections) {
			m.collectionsService.ToggleCollection(m.cursor)
			m.collections = m.collectionsService.GetCollections()
			m.updateViewportContent()
			// Send message to notify about collection changes
			return m, tea.Cmd(func() tea.Msg {
				return CollectionsUpdatedMsg{
					SelectedCollections: m.collectionsService.GetSelectedCollections(),
				}
			})
		}
		return m, nil
	case "ctrl+a": // Select all
		m.collectionsService.SelectAll()
		m.collections = m.collectionsService.GetCollections()
		m.updateViewportContent()
		// Send message to notify about collection changes
		return m, tea.Cmd(func() tea.Msg {
			return CollectionsUpdatedMsg{
				SelectedCollections: m.collectionsService.GetSelectedCollections(),
			}
		})
	case "ctrl+d": // Deselect all
		m.collectionsService.DeselectAll()
		m.collections = m.collectionsService.GetCollections()
		m.updateViewportContent()
		// Send message to notify about collection changes
		return m, tea.Cmd(func() tea.Msg {
			return CollectionsUpdatedMsg{
				SelectedCollections: m.collectionsService.GetSelectedCollections(),
			}
		})
	case "r": // Refresh collections
		if m.connected {
			m.loading = true
			return m, m.loadCollections(m.ctx)
		}
		return m, nil
	}
	return m, nil
}

// handleConfigInput handles text input when editing configuration fields
func (m Model) handleConfigInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Save the field value
		if err := m.setConfigFieldValue(m.configInput); err != nil {
			m.configMessage = fmt.Sprintf("Error: %v", err)
		} else {
			m.configMessage = "Configuration saved"
			return m, m.saveConfiguration()
		}
		m.editingConfig = false
		return m, nil
	case "esc":
		// Cancel editing
		m.editingConfig = false
		m.configInput = ""
		m.configMessage = ""
		return m, nil
	case "backspace":
		if len(m.configInput) > 0 && m.configCursor > 0 {
			m.configInput = m.configInput[:m.configCursor-1] + m.configInput[m.configCursor:]
			m.configCursor--
		}
		return m, nil
	case "left":
		if m.configCursor > 0 {
			m.configCursor--
		}
		return m, nil
	case "right":
		if m.configCursor < len(m.configInput) {
			m.configCursor++
		}
		return m, nil
	default:
		// Regular character input
		if len(msg.String()) == 1 {
			char := msg.String()
			m.configInput = m.configInput[:m.configCursor] + char + m.configInput[m.configCursor:]
			m.configCursor++
		}
		return m, nil
	}
}

// getCurrentConfigFieldValue returns the current value of the active configuration field
func (m Model) getCurrentConfigFieldValue() string {
	switch m.activeConfigField {
	case EmbeddingModelField:
		return m.editConfig.EmbeddingModel
	case ChromaDBURLField:
		return m.editConfig.ChromaDBURL
	case ChromaDBDistanceField:
		return fmt.Sprintf("%.2f", m.editConfig.ChromaDBDistance)
	case MaxDocumentsField:
		return fmt.Sprintf("%d", m.editConfig.MaxDocuments)
	default:
		return ""
	}
}

// setConfigFieldValue sets the value of the active configuration field
func (m *Model) setConfigFieldValue(value string) error {
	switch m.activeConfigField {
	case EmbeddingModelField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("embedding model cannot be empty")
		}
		m.editConfig.EmbeddingModel = strings.TrimSpace(value)
	case ChromaDBURLField:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("ChromaDB URL cannot be empty")
		}
		m.editConfig.ChromaDBURL = strings.TrimSpace(value)
	case ChromaDBDistanceField:
		distance, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return fmt.Errorf("distance must be a number")
		}
		if distance < 0 || distance > 2 {
			return fmt.Errorf("distance must be between 0 and 2")
		}
		m.editConfig.ChromaDBDistance = distance
	case MaxDocumentsField:
		maxDocs, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("max documents must be a number")
		}
		if maxDocs < 1 {
			return fmt.Errorf("max documents must be at least 1")
		}
		m.editConfig.MaxDocuments = maxDocs
	}
	return nil
}

// saveConfiguration saves the current edit configuration
func (m Model) saveConfiguration() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Update the main config
		m.config.RAGEnabled = m.editConfig.RAGEnabled
		m.config.EmbeddingModel = m.editConfig.EmbeddingModel
		m.config.ChromaDBURL = m.editConfig.ChromaDBURL
		m.config.ChromaDBDistance = m.editConfig.ChromaDBDistance
		m.config.MaxDocuments = m.editConfig.MaxDocuments
		
		// Save to file
		if err := m.config.Save(); err != nil {
			return ConfigUpdatedMsg{Config: m.config} // Still update even if save failed
		}
		
		return ConfigUpdatedMsg{Config: m.config}
	})
}
