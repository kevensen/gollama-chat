package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevensen/gollama-chat/internal/configuration"
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

// Model represents the RAG collections tab model
type Model struct {
	config             *configuration.Config
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
}

// NewModel creates a new RAG collections model
func NewModel(ctx context.Context, config *configuration.Config) Model {
	vp := viewport.New(0, 0)
	vp.YPosition = 0

	return Model{
		config:             config,
		collectionsService: NewCollectionsService(config),
		viewport:           vp,
		cursor:             0,
		collections:        make([]Collection, 0),
		connected:          false,
		loading:            true,
		ctx:                ctx,
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

		// Calculate viewport size (leave room for header and footer)
		headerHeight := 5 // Title + connection status + instructions
		footerHeight := 3 // Selection count + controls
		availableHeight := m.height - headerHeight - footerHeight
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
		}

		// Only handle other keys when not loading
		if m.loading {
			return m, nil
		}

		// Handle navigation and selection keys
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.updateViewportScroll()
				m.updateViewportContent()                                              // Add this to refresh the display
				m.error = fmt.Sprintf("DEBUG: Up pressed, cursor now at %d", m.cursor) // Debug
			}
			return m, nil // Prevent viewport from handling this key
		case "down", "j":
			if m.cursor < len(m.collections)-1 {
				m.cursor++
				m.updateViewportScroll()
				m.updateViewportContent()                                                // Add this to refresh the display
				m.error = fmt.Sprintf("DEBUG: Down pressed, cursor now at %d", m.cursor) // Debug
			}
			return m, nil // Prevent viewport from handling this key
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

	case connectionTestMsg:
		m.loading = false
		m.connected = msg.connected
		if msg.err != nil {
			m.error = fmt.Sprintf("Connection failed: %v", msg.err)
			m.collections = make([]Collection, 0)
		} else {
			m.error = ""
			if m.connected {
				return m, m.loadCollections(m.ctx)
			}
		}
		m.updateViewportContent()

	case collectionsLoadedMsg:
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
			// Send message to notify about collection changes (all collections are selected by default)
			return m, tea.Cmd(func() tea.Msg {
				return CollectionsUpdatedMsg{
					SelectedCollections: m.collectionsService.GetSelectedCollections(),
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

	// Header section
	var header strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4")).
		PaddingBottom(1)
	header.WriteString(titleStyle.Render("RAG Collections"))
	header.WriteString("\n\n")

	// Connection status
	connectionStyle := lipgloss.NewStyle()
	if m.connected {
		connectionStyle = connectionStyle.Foreground(lipgloss.Color("2")) // Green
		header.WriteString(connectionStyle.Render("✓ Connected to ChromaDB"))
	} else {
		connectionStyle = connectionStyle.Foreground(lipgloss.Color("1")) // Red
		header.WriteString(connectionStyle.Render("✗ Not connected to ChromaDB"))
		if m.error != "" {
			header.WriteString(fmt.Sprintf(" - %s", m.error))
		}
	}
	header.WriteString("\n\n")

	// Instructions
	if m.connected && !m.loading {
		instructionsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
		header.WriteString(instructionsStyle.Render("Use ↑/↓ to navigate, Space/Enter to toggle, Ctrl+A to select all, Ctrl+D to deselect all, R to refresh"))
	} else if !m.connected {
		instructionsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
		header.WriteString(instructionsStyle.Render("Press T to test connection, C to show config, U to refresh config, X to stop loading"))
	}
	header.WriteString("\n\n")

	// Main content area
	var content string
	if m.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("6"))
		content = loadingStyle.Render("Loading...")
	} else if !m.connected {
		content = "ChromaDB connection required to view collections.\nEnsure ChromaDB is running and URL is configured in Settings."
	} else if len(m.collections) == 0 {
		content = "No collections found in ChromaDB."
	} else {
		content = m.viewport.View()
	}

	// Footer section
	var footer strings.Builder
	if m.connected && len(m.collections) > 0 {
		selectedCount := m.collectionsService.GetSelectedCount()
		totalCount := len(m.collections)

		countStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")).
			Bold(true)
		footer.WriteString(countStyle.Render(fmt.Sprintf("Selected: %d/%d collections", selectedCount, totalCount)))
	}

	// Combine all sections
	var output strings.Builder
	output.WriteString(header.String())
	output.WriteString(content)
	if footer.Len() > 0 {
		output.WriteString("\n\n")
		output.WriteString(footer.String())
	}

	return output.String()
}

// testConnection starts connection testing
func (m Model) testConnection() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		err := m.collectionsService.TestConnection()
		return connectionTestMsg{
			connected: m.collectionsService.IsConnected(),
			err:       err,
		}
	})
}

// loadCollections starts collections loading
func (m Model) loadCollections(ctx context.Context) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		err := m.collectionsService.LoadCollections(ctx)
		return collectionsLoadedMsg{
			collections: m.collectionsService.GetCollections(),
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
