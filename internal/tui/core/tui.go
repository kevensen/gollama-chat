package core

import (
	"context"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/chat"
	configTab "github.com/kevensen/gollama-chat/internal/tui/tabs/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/configuration/utils/connection"
	ragTab "github.com/kevensen/gollama-chat/internal/tui/tabs/rag"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	// VISIBILITY PRIORITY: Always ensure tabs and footer are visible
	// Priority 1: Tab bar (always gets 1 line)
	tabBar := m.renderTabBar()

	// Priority 2: Footer/toolbar (always gets 1 line)
	footer := m.renderFooter()

	// For extremely small terminals, just show tabs and footer
	if m.height <= 2 {
		if m.height == 1 {
			// Only room for tabs
			return tabBar
		}
		if m.height == 2 {
			// Room for tabs + footer
			return lipgloss.JoinVertical(lipgloss.Left, tabBar, footer)
		}
	}

	// Calculate remaining space for content after reserving space for tabs and footer
	tabBarHeight := 1 // Always reserve 1 line for tabs
	footerHeight := 1 // Always reserve 1 line for footer
	contentHeight := m.height - tabBarHeight - footerHeight

	// If no room for content, show minimal placeholder
	if contentHeight < 1 {
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, footer)
	}

	// Content area - gets whatever space is left after tabs and footer
	// Update the active model with the correct content height first
	var content string
	switch m.activeTab {
	case ChatTab:
		// Update chat model with constrained height for proper layout
		chatModel, _ := m.chatModel.Update(tea.WindowSizeMsg{
			Width:  m.width,
			Height: contentHeight + 2, // Add 2 for tabs and footer that chat model accounts for
		})
		m.chatModel = chatModel.(chat.Model)
		content = m.chatModel.View()
	case ConfigTab:
		content = m.configModel.View()
	case RAGTab:
		content = m.ragModel.View()
	}

	// Style content to fit available space
	var styledContent string
	if m.activeTab == ChatTab {
		// Chat tab now manages its own height correctly, just ensure width
		contentStyle := lipgloss.NewStyle().
			Width(m.width)
		styledContent = contentStyle.Render(content)
	} else {
		// Config and RAG tabs manage their own height with containers
		styledContent = content
	}

	// Return the layout with guaranteed tab and footer visibility
	return lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		styledContent,
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

	// Create compact tab names based on available width for maximum visibility
	var tabNames []string
	if m.width >= 25 {
		// Full tab names for reasonable width
		tabNames = []string{"Chat", "Settings", "RAG Collections"}
	} else if m.width >= 10 {
		// Short names for narrow terminals
		tabNames = []string{"C", "S", "R"}
	} else if m.width >= 6 {
		// Ultra-compact for very narrow terminals
		tabNames = []string{"C", "S", "R"}
	} else {
		// Minimal representation for extremely narrow terminals (width 2-5)
		tabNames = []string{"C"}
	}

	for i, tab := range tabNames {
		if i >= len(m.tabs) {
			break // Safety check
		}

		// Adjust spacing based on available width
		var tabText string
		if m.width >= 10 {
			tabText = " " + tab + " " // Normal spacing
		} else {
			tabText = tab // No spacing for very narrow terminals
		}

		if Tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(tabText))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(tabText))
		}
	}

	// Always ensure we have content to render with progressive fallbacks
	tabContent := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	if tabContent == "" {
		// Progressive fallbacks for extreme cases
		if m.width >= 15 {
			tabContent = "Chat Settings RAG"
		} else if m.width >= 7 {
			tabContent = "C S R"
		} else if m.width >= 3 {
			tabContent = "CSR"
		} else {
			tabContent = "C" // Absolute minimum
		}
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

	// Progressive help text based on available width for maximum visibility
	var helpText string
	if m.width >= 80 {
		// Full help with tab-specific commands
		helpText = "Tab/Shift+Tab: Switch tabs • Ctrl+C/q: Quit"
		switch m.activeTab {
		case ChatTab:
			helpText += " • Enter: Send • ↑/↓: Scroll • Ctrl+S: System Prompt"
		case ConfigTab:
			helpText += " • Enter: Edit • Esc: Cancel"
		case RAGTab:
			helpText += " • Space: Toggle • ↑/↓: Navigate • R: Refresh"
		}
	} else if m.width >= 50 {
		// Medium detail
		helpText = "Tab/Shift+Tab: Switch tabs • Ctrl+C/q: Quit"
	} else if m.width >= 25 {
		// Basic help
		helpText = "Tab: Switch • Ctrl+C: Quit"
	} else if m.width >= 15 {
		// Minimal help
		helpText = "Tab:Switch Q:Quit"
	} else if m.width >= 8 {
		// Ultra-minimal
		helpText = "Tab Q"
	} else {
		// Extremely narrow - just show essential
		helpText = "Tab"
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
