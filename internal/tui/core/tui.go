package core

import (
	"context"
	"time"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
	"github.com/kevensen/gollama-chat/internal/tooling"
	mcpManager "github.com/kevensen/gollama-chat/internal/tooling/mcp"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/chat"
	configTab "github.com/kevensen/gollama-chat/internal/tui/tabs/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/configuration/utils/connection"
	mcpTab "github.com/kevensen/gollama-chat/internal/tui/tabs/mcp"
	ragTab "github.com/kevensen/gollama-chat/internal/tui/tabs/rag"
	toolsTab "github.com/kevensen/gollama-chat/internal/tui/tabs/tools"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// syncRAGCollectionsMsg is sent to trigger RAG collection synchronization
type syncRAGCollectionsMsg struct{}

// Tab represents the different tabs in the application
type Tab int

const (
	ChatTab Tab = iota
	ConfigTab
	RAGTab
	ToolsTab
	MCPTab
)

// Model represents the main TUI model
type Model struct {
	ctx         context.Context
	config      *configuration.Config
	mcpManager  *mcpManager.Manager
	activeTab   Tab
	tabs        []string
	chatModel   chat.Model
	configModel configTab.Model
	ragModel    ragTab.Model
	toolsModel  toolsTab.Model
	mcpModel    mcpTab.Model
	width       int
	height      int
}

// NewModel creates a new TUI model
func NewModel(ctx context.Context, config *configuration.Config) *Model {
	logger := logging.WithComponent("tui-core")
	logger.Debug("Creating new TUI model")

	// Create shared MCP manager
	sharedMCPManager := mcpManager.NewManager(config)
	logger.Debug("Created MCP manager")

	// Set the MCP manager on the DefaultRegistry so it can be used by the chat model
	tooling.DefaultRegistry.SetMCPManager(sharedMCPManager)
	logger.Debug("Set MCP manager on DefaultRegistry")

	logger.Debug("Initializing tab models")
	model := &Model{
		ctx:         ctx,
		config:      config,
		mcpManager:  sharedMCPManager,
		activeTab:   ChatTab,
		tabs:        []string{"Chat", "Settings", "RAG Collections", "Tools", "MCP Servers"},
		chatModel:   chat.NewModel(ctx, config),
		configModel: configTab.NewModel(config),
		ragModel:    ragTab.NewModel(ctx, config),
		toolsModel:  toolsTab.NewModel(ctx, config, sharedMCPManager),
		mcpModel:    mcpTab.NewModel(ctx, config, sharedMCPManager),
	}

	logger.Info("TUI model created successfully")
	return model
}

// Init initializes the TUI model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.chatModel.Init(),
		m.configModel.Init(),
		m.ragModel.Init(),
		m.toolsModel.Init(),
		m.mcpModel.Init(),
		startMCPServersCmd(m.ctx, m.mcpManager),
		// Sync RAG collections after initialization
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return syncRAGCollectionsMsg{}
		}),
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

		toolsModel, toolsCmd := m.toolsModel.Update(tea.WindowSizeMsg{
			Width:  m.width,
			Height: m.height,
		})
		m.toolsModel = toolsModel.(toolsTab.Model)
		if toolsCmd != nil {
			cmds = append(cmds, toolsCmd)
		}

		mcpModel, mcpCmd := m.mcpModel.Update(tea.WindowSizeMsg{
			Width:  m.width,
			Height: m.height,
		})
		m.mcpModel = mcpModel
		if mcpCmd != nil {
			cmds = append(cmds, mcpCmd)
		}

	case tea.KeyMsg:
		logger := logging.WithComponent("tui-core")
		switch msg.String() {
		case "ctrl+c":
			logger.Info("User requested quit")
			return m, tea.Quit
		case "tab":
			// Check if MCP tab is active and in form mode - if so, let it handle tab navigation
			if m.activeTab == MCPTab && m.mcpModel.IsInFormMode() {
				mcpModel, mcpCmd := m.mcpModel.Update(msg)
				m.mcpModel = mcpModel
				return m, mcpCmd
			}

			// Switch tabs
			oldTab := m.activeTab
			m.activeTab = (m.activeTab + 1) % Tab(len(m.tabs))
			logger.Debug("Tab switch", "from", oldTab, "to", m.activeTab)

			// Trigger initialization when switching to RAG tab
			if oldTab != RAGTab && m.activeTab == RAGTab {
				logger.Debug("Initializing RAG tab")
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
			// Trigger initialization when switching to Tools tab
			if oldTab != ToolsTab && m.activeTab == ToolsTab {
				logger.Debug("Initializing Tools tab")
				// Refresh tools when entering Tools tab
				toolsModel, toolsCmd := m.toolsModel.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
				m.toolsModel = toolsModel.(toolsTab.Model)
				initCmd := m.toolsModel.Init()
				if toolsCmd != nil && initCmd != nil {
					cmd = tea.Batch(toolsCmd, initCmd)
				} else if toolsCmd != nil {
					cmd = toolsCmd
				} else if initCmd != nil {
					cmd = initCmd
				}
			}
			// Trigger initialization when switching to MCP tab
			if oldTab != MCPTab && m.activeTab == MCPTab {
				logger.Debug("Initializing MCP tab")
				// Refresh servers when entering MCP tab
				mcpModel, mcpCmd := m.mcpModel.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
				m.mcpModel = mcpModel
				initCmd := m.mcpModel.Init()
				if mcpCmd != nil && initCmd != nil {
					cmd = tea.Batch(mcpCmd, initCmd)
				} else if mcpCmd != nil {
					cmd = mcpCmd
				} else if initCmd != nil {
					cmd = initCmd
				}
			}
			// Sync selected collections when switching to Chat tab
			if m.activeTab == ChatTab {
				logger.Debug("Syncing RAG collections for Chat tab")
				m.syncRAGCollections()
			}
		case "shift+tab":
			// Check if MCP tab is active and in form mode - if so, let it handle shift+tab navigation
			if m.activeTab == MCPTab && m.mcpModel.IsInFormMode() {
				mcpModel, mcpCmd := m.mcpModel.Update(msg)
				m.mcpModel = mcpModel
				return m, mcpCmd
			}

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
			// Trigger initialization when switching to Tools tab
			if oldTab != ToolsTab && m.activeTab == ToolsTab {
				// Refresh tools when entering Tools tab
				toolsModel, toolsCmd := m.toolsModel.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
				m.toolsModel = toolsModel.(toolsTab.Model)
				initCmd := m.toolsModel.Init()
				if toolsCmd != nil && initCmd != nil {
					cmd = tea.Batch(toolsCmd, initCmd)
				} else if toolsCmd != nil {
					cmd = toolsCmd
				} else if initCmd != nil {
					cmd = initCmd
				}
			}
			// Trigger initialization when switching to MCP tab
			if oldTab != MCPTab && m.activeTab == MCPTab {
				// Refresh servers when entering MCP tab
				mcpModel, mcpCmd := m.mcpModel.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
				m.mcpModel = mcpModel
				initCmd := m.mcpModel.Init()
				if mcpCmd != nil && initCmd != nil {
					cmd = tea.Batch(mcpCmd, initCmd)
				} else if mcpCmd != nil {
					cmd = mcpCmd
				} else if initCmd != nil {
					cmd = initCmd
				}
			}
			// Sync selected collections when switching to Chat tab
			if m.activeTab == ChatTab {
				m.syncRAGCollections()
			}
		case "shift+enter":
			// Check if MCP tab is active and in form mode - if so, let it handle shift+enter navigation
			if m.activeTab == MCPTab && m.mcpModel.IsInFormMode() {
				mcpModel, mcpCmd := m.mcpModel.Update(msg)
				m.mcpModel = mcpModel
				return m, mcpCmd
			}
			// Otherwise, forward to the active tab as usual
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
			case ToolsTab:
				toolsModel, toolsCmd := m.toolsModel.Update(msg)
				m.toolsModel = toolsModel.(toolsTab.Model)
				cmd = toolsCmd
			case MCPTab:
				mcpModel, mcpCmd := m.mcpModel.Update(msg)
				m.mcpModel = mcpModel
				cmd = mcpCmd
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
			case ToolsTab:
				toolsModel, toolsCmd := m.toolsModel.Update(msg)
				m.toolsModel = toolsModel.(toolsTab.Model)
				cmd = toolsCmd
			case MCPTab:
				mcpModel, mcpCmd := m.mcpModel.Update(msg)
				m.mcpModel = mcpModel
				cmd = mcpCmd
			}
		}

	default:
		// Handle RAG collection synchronization
		if _, isSyncRAGMsg := msg.(syncRAGCollectionsMsg); isSyncRAGMsg {
			m.syncRAGCollections()
			return m, nil
		}

		// Handle configuration updates
		if configMsg, isConfigUpdate := msg.(ragTab.ConfigUpdatedMsg); isConfigUpdate {
			// Update the main config
			m.config = configMsg.Config

			// Update the chat model with the new configuration (handles system prompt precedence)
			m.chatModel.UpdateFromConfiguration(configMsg.Config)

			// Update the RAG model with the new configuration
			ragModel, ragCmd := m.ragModel.Update(configMsg)
			m.ragModel = ragModel.(ragTab.Model)
			cmd = ragCmd
		} else if collectionsMsg, isCollectionsUpdate := msg.(ragTab.CollectionsUpdatedMsg); isCollectionsUpdate {
			logger := logging.WithComponent("tui-core")
			logger.Info("Received collections update message",
				"selected_collections", collectionsMsg.SelectedCollections,
				"count", len(collectionsMsg.SelectedCollections))

			// Handle collection selection changes
			selectedCollectionsMap := make(map[string]bool)
			for _, collectionName := range collectionsMsg.SelectedCollections {
				selectedCollectionsMap[collectionName] = true
			}

			// Update the chat model's RAG service
			ragService := m.chatModel.GetRAGService()
			if ragService != nil {
				ragService.UpdateSelectedCollections(selectedCollectionsMap)
				logger.Info("Updated RAG service with selected collections",
					"collections_map", selectedCollectionsMap)
			} else {
				logger.Warn("RAG service is nil, cannot update collections")
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
			case ToolsTab:
				toolsModel, toolsCmd := m.toolsModel.Update(msg)
				m.toolsModel = toolsModel.(toolsTab.Model)
				cmd = toolsCmd
			case MCPTab:
				mcpModel, mcpCmd := m.mcpModel.Update(msg)
				m.mcpModel = mcpModel
				cmd = mcpCmd
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
	case ToolsTab:
		content = m.toolsModel.View()
	case MCPTab:
		content = m.mcpModel.View()
	}

	// Style content to fit available space
	var styledContent string
	if m.activeTab == ChatTab {
		// Chat tab now manages its own height correctly, just ensure width
		contentStyle := lipgloss.NewStyle().
			Width(m.width)
		styledContent = contentStyle.Render(content)
	} else {
		// Config, RAG, and Tools tabs manage their own height with containers
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
	if m.width >= 40 {
		// Full tab names for reasonable width
		tabNames = []string{"Chat", "Settings", "RAG Collections", "Tools", "MCP Servers"}
	} else if m.width >= 20 {
		// Medium names for moderate width
		tabNames = []string{"Chat", "Config", "RAG", "Tools", "MCP"}
	} else if m.width >= 12 {
		// Short names for narrow terminals
		tabNames = []string{"C", "S", "R", "T", "M"}
	} else if m.width >= 8 {
		// Ultra-compact for very narrow terminals
		tabNames = []string{"C", "S", "R", "T", "M"}
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
		if m.width >= 25 {
			tabContent = "Chat Settings RAG Tools MCP"
		} else if m.width >= 12 {
			tabContent = "C S R T M"
		} else if m.width >= 5 {
			tabContent = "CSRTM"
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
		helpText = "Tab/Shift+Tab: Switch tabs • Ctrl+C: Quit"
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
		helpText = "Tab/Shift+Tab: Switch tabs • Ctrl+C: Quit"
	} else if m.width >= 25 {
		// Basic help
		helpText = "Tab: Switch • Ctrl+C: Quit"
	} else if m.width >= 15 {
		// Minimal help
		helpText = "Tab:Switch Ctrl+C:Quit"
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
	logger := logging.WithComponent("tui-core")

	// Get the selected collections from the RAG tab
	selectedCollectionNames := m.ragModel.GetSelectedCollections()

	logger.Info("Syncing RAG collections",
		"rag_tab_collections", selectedCollectionNames,
		"count", len(selectedCollectionNames))

	// If the RAG tab has no collections selected, don't override the RAG service
	// (it might have auto-selected collections during initialization)
	if len(selectedCollectionNames) == 0 {
		logger.Info("RAG tab has no collections selected, preserving RAG service selections")
		return
	}

	// Convert to the map format expected by UpdateSelectedCollections
	selectedCollectionsMap := make(map[string]bool)
	for _, collectionName := range selectedCollectionNames {
		selectedCollectionsMap[collectionName] = true
	}

	// Update the chat model's RAG service with the selected collections
	ragService := m.chatModel.GetRAGService()
	if ragService != nil {
		ragService.UpdateSelectedCollections(selectedCollectionsMap)
		logger.Info("Updated RAG service with collections from RAG tab",
			"collections_map", selectedCollectionsMap)
	}
}

// startMCPServersCmd creates a command to start enabled MCP servers
func startMCPServersCmd(ctx context.Context, manager *mcpManager.Manager) tea.Cmd {
	return func() tea.Msg {
		// Use a background context for server startup
		if err := manager.StartEnabledServers(ctx); err != nil {
			// For now, we'll silently handle the error
			// In the future, we might want to create a specific message type
			// to handle MCP startup errors in the UI
			return nil
		}
		return nil
	}
}
