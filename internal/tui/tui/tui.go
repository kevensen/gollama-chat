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

	// Calculate available height for content
	contentHeight := m.height - lipgloss.Height(tabBar) - lipgloss.Height(footer)

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

	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Padding(0, 2)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Background(lipgloss.Color("235")).
		Padding(0, 2)

	for i, tab := range m.tabs {
		if Tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(tab))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(tab))
		}
	}

	tabBarStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Width(m.width)

	return tabBarStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
}

// renderFooter renders the footer with help text
func (m Model) renderFooter() string {
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Width(m.width)

	helpText := "Tab/Shift+Tab: Switch tabs • Ctrl+C/q: Quit"

	// Add tab-specific help
	switch m.activeTab {
	case ChatTab:
		helpText += " • Enter: Send • ↑/↓: Scroll line • PgUp/PgDn: Scroll page • Ctrl+L: Clear Chat • Ctrl+S: Toggle System Prompt"
	case ConfigTab:
		helpText += " • Enter: Edit & auto-save • Esc: Cancel"
	}

	return footerStyle.Render(helpText)
}
