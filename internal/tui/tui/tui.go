package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/chat"
	configTab "github.com/kevensen/gollama-chat/internal/tui/tabs/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/configuration/utils/connection"
	ragTab "github.com/kevensen/gollama-chat/internal/tui/tabs/rag"
)

// Tab represents the different tabs in the application
type Tab int

const (
	ChatTab Tab = iota
	ConfigTab
	RAGTab
)

// Model represents the main TUI model
type Model struct {
	config      *configuration.Config
	activeTab   Tab
	tabs        []string
	chatModel   chat.Model
	configModel configTab.Model
	ragModel    ragTab.Model
	width       int
	height      int
}

// NewModel creates a new TUI model
func NewModel(ctx context.Context, config *configuration.Config) Model {
	return Model{
		config:      config,
		activeTab:   ChatTab,
		tabs:        []string{"Chat", "Settings", "RAG Collections"},
		chatModel:   chat.NewModel(ctx, config),
		configModel: configTab.NewModel(config),
		ragModel:    ragTab.NewModel(ctx, config),
	}
}

// Init initializes the TUI model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.chatModel.Init(),
		m.configModel.Init(),
		m.ragModel.Init(),
	)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Use 90% of terminal dimensions for better visibility (reduced from 95%)
		m.width = int(float64(msg.Width) * 0.90)
		m.height = int(float64(msg.Height) * 0.90)

		// Update child models with new size
		chatModel, chatCmd := m.chatModel.Update(tea.WindowSizeMsg{
			Width:  m.width,
			Height: m.height,
		})
		m.chatModel = chatModel.(chat.Model)
		if chatCmd != nil {
			cmds = append(cmds, chatCmd)
		}

		configModel, configCmd := m.configModel.Update(tea.WindowSizeMsg{
			Width:  m.width,
			Height: m.height,
		})
		m.configModel = configModel.(configTab.Model)
		if configCmd != nil {
			cmds = append(cmds, configCmd)
		}

		ragModel, ragCmd := m.ragModel.Update(tea.WindowSizeMsg{
			Width:  m.width,
			Height: m.height,
		})
		m.ragModel = ragModel.(ragTab.Model)
		if ragCmd != nil {
			cmds = append(cmds, ragCmd)
		}

	case tea.KeyMsg:
		// ULTRA-FAST PATH: Handle ASCII input with proper encapsulation
		if m.activeTab == ChatTab {
			// Handle space character specifically
			if msg.String() == " " {
				if m.chatModel.HandleFastInputChar(' ') {
					return m, nil
				}
			}

			// Use Runes directly for other ASCII characters
			if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
				char := msg.Runes[0]
				if m.chatModel.HandleFastInputChar(char) {
					return m, nil
				}
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			// Switch tabs
			oldTab := m.activeTab
			m.activeTab = (m.activeTab + 1) % Tab(len(m.tabs))
			// Trigger initialization when switching to RAG tab
			if oldTab != RAGTab && m.activeTab == RAGTab {
				// Test connection when entering RAG tab
				ragModel, ragCmd := m.ragModel.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
				m.ragModel = ragModel.(ragTab.Model)
				initCmd := m.ragModel.Init()
				if ragCmd != nil && initCmd != nil {
					cmd = tea.Batch(ragCmd, initCmd)
				} else if ragCmd != nil {
					cmd = ragCmd
				} else if initCmd != nil {
					cmd = initCmd
				}
			}
			// Sync selected collections when switching to Chat tab
			if m.activeTab == ChatTab {
				m.syncRAGCollections()
			}
		case "shift+tab":
			// Switch tabs in reverse
			oldTab := m.activeTab
			m.activeTab = (m.activeTab - 1 + Tab(len(m.tabs))) % Tab(len(m.tabs))
			// Trigger initialization when switching to RAG tab
			if oldTab != RAGTab && m.activeTab == RAGTab {
				// Test connection when entering RAG tab
				ragModel, ragCmd := m.ragModel.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
				m.ragModel = ragModel.(ragTab.Model)
				initCmd := m.ragModel.Init()
				if ragCmd != nil && initCmd != nil {
					cmd = tea.Batch(ragCmd, initCmd)
				} else if ragCmd != nil {
					cmd = ragCmd
				} else if initCmd != nil {
					cmd = initCmd
				}
			}
			// Sync selected collections when switching to Chat tab
			if m.activeTab == ChatTab {
				m.syncRAGCollections()
			}
		default:
			// Forward key messages to the active tab
			switch m.activeTab {
			case ChatTab:
				chatModel, chatCmd := m.chatModel.Update(msg)
				m.chatModel = chatModel.(chat.Model)
				cmd = chatCmd
			case ConfigTab:
				configModel, configCmd := m.configModel.Update(msg)
				m.configModel = configModel.(configTab.Model)
				cmd = configCmd
			case RAGTab:
				ragModel, ragCmd := m.ragModel.Update(msg)
				m.ragModel = ragModel.(ragTab.Model)
				cmd = ragCmd
			}
		}

	default:
		// Handle configuration updates
		if configMsg, isConfigUpdate := msg.(ragTab.ConfigUpdatedMsg); isConfigUpdate {
			// Update the main config
			m.config = configMsg.Config

			// Update the RAG model with the new configuration
			ragModel, ragCmd := m.ragModel.Update(configMsg)
			m.ragModel = ragModel.(ragTab.Model)
			cmd = ragCmd
		} else if collectionsMsg, isCollectionsUpdate := msg.(ragTab.CollectionsUpdatedMsg); isCollectionsUpdate {
			// Handle collection selection changes
			selectedCollectionsMap := make(map[string]bool)
			for _, collectionName := range collectionsMsg.SelectedCollections {
				selectedCollectionsMap[collectionName] = true
			}

			// Update the chat model's RAG service
			ragService := m.chatModel.GetRAGService()
			if ragService != nil {
				ragService.UpdateSelectedCollections(selectedCollectionsMap)
			}

			// Still forward the message to the RAG tab
			ragModel, ragCmd := m.ragModel.Update(msg)
			m.ragModel = ragModel.(ragTab.Model)
			cmd = ragCmd
		} else if _, isConnectionMsg := msg.(connection.CheckMsg); isConnectionMsg {
			// Check if this is a ConnectionCheckMsg and route it to config tab
			configModel, configCmd := m.configModel.Update(msg)
			m.configModel = configModel.(configTab.Model)
			cmd = configCmd
		} else {
			// Forward other messages to the active tab
			switch m.activeTab {
			case ChatTab:
				chatModel, chatCmd := m.chatModel.Update(msg)
				m.chatModel = chatModel.(chat.Model)
				cmd = chatCmd
			case ConfigTab:
				configModel, configCmd := m.configModel.Update(msg)
				m.configModel = configModel.(configTab.Model)
				cmd = configCmd
			case RAGTab:
				ragModel, ragCmd := m.ragModel.Update(msg)
				m.ragModel = ragModel.(ragTab.Model)
				cmd = ragCmd
			}
		}
	}

	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m Model) View() string {
	// For very small terminals, prioritize tabs and minimal content
	if m.height < 3 {
		// In extreme cases, just show tabs
		if m.height == 1 {
			return m.renderTabBar()
		}
		// With 2 lines, show tabs and minimal content
		if m.height == 2 {
			tabBar := m.renderTabBar()
			return lipgloss.JoinVertical(lipgloss.Left, tabBar, "...")
		}
	}

	// Tab bar
	tabBar := m.renderTabBar()

	// Content area
	var content string
	switch m.activeTab {
	case ChatTab:
		content = m.chatModel.View()
	case ConfigTab:
		content = m.configModel.View()
	case RAGTab:
		content = m.ragModel.View()
	}

	// Footer with help
	footer := m.renderFooter()

	// Calculate available height for content with minimum constraints
	tabBarHeight := 1 // Always reserve 1 line for tabs
	footerHeight := 1 // Always reserve 1 line for footer

	// Calculate content height, ensuring it's never negative
	contentHeight := m.height - tabBarHeight - footerHeight
	if contentHeight < 1 {
		contentHeight = 1 // Ensure at least 1 line for content
	}

	// Style the content area
	contentStyle := lipgloss.NewStyle().
		Height(contentHeight).
		Width(m.width)

	// Return the main content without complex centering
	return lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		contentStyle.Render(content),
		footer,
	)
}

// renderTabBar renders the tab bar
func (m Model) renderTabBar() string {
	var tabs []string

	// Use minimal styles to ensure tabs fit in constrained space
	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).     // Bright white
		Background(lipgloss.Color("#8A7FD8")) // Purple-blue to match chat border

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")). // Light gray
		Background(lipgloss.Color("0"))  // Black

	// Create compact tab names for minimal space scenarios
	tabNames := []string{"Chat", "Config", "RAG"}
	if m.width < 30 {
		tabNames = []string{"C", "S", "R"} // Single letter tabs for very narrow terminals
	}

	for i, tab := range tabNames {
		if i >= len(m.tabs) {
			break // Safety check
		}

		tabText := " " + tab + " " // Add minimal spacing
		if Tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(tabText))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(tabText))
		}
	}

	// Always ensure we have content to render
	tabContent := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	if tabContent == "" {
		tabContent = " Chat Settings RAG " // Fallback content
	}

	// Create a simple style that guarantees visibility
	tabBarStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("0")). // Black background
		Width(m.width)

	return tabBarStyle.Render(tabContent)
}

// renderFooter renders the footer with help text
func (m Model) renderFooter() string {
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Background(lipgloss.Color("235")).
		Width(m.width)

	// Simplified help text for small terminals
	helpText := "Tab: Switch • Ctrl+C: Quit"

	// Add more detailed help only if we have enough width
	if m.width > 50 {
		helpText = "Tab/Shift+Tab: Switch tabs • Ctrl+C/q: Quit"

		// Add tab-specific help only for wider terminals
		if m.width > 80 {
			switch m.activeTab {
			case ChatTab:
				helpText += " • Enter: Send • ↑/↓: Scroll"
			case ConfigTab:
				helpText += " • Enter: Edit • Esc: Cancel"
			}
		}
	}

	return footerStyle.Render(helpText)
}

// syncRAGCollections synchronizes the selected collections from RAG tab to the chat model's RAG service
func (m *Model) syncRAGCollections() {
	// Get the selected collections from the RAG tab
	selectedCollectionNames := m.ragModel.GetSelectedCollections()

	// Convert to the map format expected by UpdateSelectedCollections
	selectedCollectionsMap := make(map[string]bool)
	for _, collectionName := range selectedCollectionNames {
		selectedCollectionsMap[collectionName] = true
	}

	// Update the chat model's RAG service with the selected collections
	ragService := m.chatModel.GetRAGService()
	if ragService != nil {
		ragService.UpdateSelectedCollections(selectedCollectionsMap)
	}
}
