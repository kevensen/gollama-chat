package tools

import (
	"context"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ollama/ollama/api"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/tooling"
)

// TrustLevel represents the level of trust for a tool
type TrustLevel int

const (
	TrustNone    TrustLevel = iota // Not trusted, will prompt for each invocation
	TrustOnce                      // Trusted for current invocation only
	TrustSession                   // Trusted for the entire session
)

func (tl TrustLevel) String() string {
	switch tl {
	case TrustNone:
		return "None"
	case TrustOnce:
		return "Once"
	case TrustSession:
		return "Session"
	default:
		return "Unknown"
	}
}

// Tool represents a tool available to the system
type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Trust       TrustLevel `json:"trust"`
	LastUsed    *time.Time `json:"last_used"`
	UsageCount  int        `json:"usage_count"`
	Source      string     `json:"source"` // "builtin", "mcp", etc.
	APITool     *api.Tool  `json:"-"`      // The actual Ollama API tool
}

// Model represents the tools tab model
type Model struct {
	config        *configuration.Config
	ctx           context.Context
	tools         []Tool
	selectedIndex int
	width         int
	height        int
	scrollOffset  int
	message       string
	messageStyle  lipgloss.Style

	// UI state
	viewMode    ViewMode
	trustPrompt *TrustPrompt
}

// ViewMode represents the current view mode of the tools tab
type ViewMode int

const (
	ViewModeList ViewMode = iota
	ViewModeTrustPrompt
)

// TrustPrompt represents a trust prompt for a tool
type TrustPrompt struct {
	Tool          *Tool
	Message       string
	SelectedLevel TrustLevel
}

// NewModel creates a new tools model
func NewModel(ctx context.Context, config *configuration.Config) Model {
	return Model{
		config:        config,
		ctx:           ctx,
		tools:         []Tool{}, // Start with empty tools list
		selectedIndex: 0,
		viewMode:      ViewModeList,
		messageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true),
	}
}

// Init initializes the tools model
func (m Model) Init() tea.Cmd {
	// Automatically refresh tools on startup to load built-in tools
	return m.refreshTools()
}

// Update handles messages and updates the tools model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ToolsRefreshedMsg:
		m.tools = msg.Tools
		if msg.Error != nil {
			m.message = "Error refreshing tools: " + msg.Error.Error()
		} else {
			m.message = "Tools refreshed successfully"
		}
		if len(m.tools) > 0 && m.selectedIndex >= len(m.tools) {
			m.selectedIndex = len(m.tools) - 1
		}

	case tea.KeyMsg:
		if m.viewMode == ViewModeTrustPrompt {
			return m.handleTrustPromptKeys(msg)
		}

		switch msg.String() {
		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "down", "j":
			if m.selectedIndex < len(m.tools)-1 {
				m.selectedIndex++
			}
		case "home":
			m.selectedIndex = 0
		case "end":
			if len(m.tools) > 0 {
				m.selectedIndex = len(m.tools) - 1
			}
		case "pgup":
			m.selectedIndex = max(0, m.selectedIndex-10)
		case "pgdown":
			m.selectedIndex = min(len(m.tools)-1, m.selectedIndex+10)
		case "enter":
			if len(m.tools) > 0 {
				return m.showTrustPrompt()
			}
		case "r":
			// Refresh tools list (for future MCP integration)
			m.message = "Refreshing tools..."
			return m, m.refreshTools()
		case "d":
			// Show details for selected tool
			if len(m.tools) > 0 {
				return m.showToolDetails()
			}
		}
	}

	return m, nil
}

// handleTrustPromptKeys handles keyboard input when in trust prompt mode
func (m Model) handleTrustPromptKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.trustPrompt == nil {
		m.viewMode = ViewModeList
		return m, nil
	}

	switch msg.String() {
	case "escape":
		m.viewMode = ViewModeList
		m.trustPrompt = nil
	case "up", "k":
		if m.trustPrompt.SelectedLevel > TrustNone {
			m.trustPrompt.SelectedLevel--
		}
	case "down", "j":
		if m.trustPrompt.SelectedLevel < TrustSession {
			m.trustPrompt.SelectedLevel++
		}
	case "enter":
		// Apply the selected trust level
		if m.selectedIndex < len(m.tools) {
			m.tools[m.selectedIndex].Trust = m.trustPrompt.SelectedLevel
			m.message = "Trust level updated for " + m.tools[m.selectedIndex].Name
		}
		m.viewMode = ViewModeList
		m.trustPrompt = nil
	}

	return m, nil
}

// showTrustPrompt shows the trust configuration prompt for the selected tool
func (m Model) showTrustPrompt() (tea.Model, tea.Cmd) {
	if m.selectedIndex >= len(m.tools) {
		return m, nil
	}

	tool := &m.tools[m.selectedIndex]
	m.trustPrompt = &TrustPrompt{
		Tool:          tool,
		Message:       "Configure trust level for " + tool.Name,
		SelectedLevel: tool.Trust,
	}
	m.viewMode = ViewModeTrustPrompt

	return m, nil
}

// showToolDetails shows detailed information about the selected tool
func (m Model) showToolDetails() (tea.Model, tea.Cmd) {
	if m.selectedIndex >= len(m.tools) {
		return m, nil
	}

	tool := m.tools[m.selectedIndex]
	details := strings.Builder{}
	details.WriteString("Tool: " + tool.Name + "\n")
	details.WriteString("Description: " + tool.Description + "\n")
	details.WriteString("Source: " + tool.Source + "\n")
	details.WriteString("Trust Level: " + tool.Trust.String() + "\n")
	details.WriteString("Usage Count: " + strconv.Itoa(tool.UsageCount) + "\n")
	if tool.LastUsed != nil {
		details.WriteString("Last Used: " + tool.LastUsed.Format(time.RFC3339) + "\n")
	}

	m.message = details.String()
	return m, nil
}

// refreshTools refreshes the list of available tools
func (m Model) refreshTools() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		var tools []Tool

		// Get built-in tools from the registry
		builtinTools := tooling.DefaultRegistry.GetAllTools()
		for _, builtinTool := range builtinTools {
			// Set default trust level based on tool type
			defaultTrust := TrustNone
			if builtinTool.Name() == "filesystem_read" {
				defaultTrust = TrustSession // Filesystem read tool is trusted by default
			}

			tool := Tool{
				Name:        builtinTool.Name(),
				Description: builtinTool.Description(),
				Trust:       defaultTrust,
				Source:      "builtin",
				UsageCount:  0,
				APITool:     builtinTool.GetAPITool(),
			}
			tools = append(tools, tool)
		}

		// TODO: In the future, this would also:
		// 1. Discover MCP servers
		// 2. Enumerate their tools
		// 3. Add them to the tools list

		return ToolsRefreshedMsg{
			Tools: tools,
			Error: nil,
		}
	})
}

// ToolsRefreshedMsg is sent when tools have been refreshed
type ToolsRefreshedMsg struct {
	Tools []Tool
	Error error
}

// View renders the tools tab
func (m Model) View() string {
	if m.viewMode == ViewModeTrustPrompt {
		return m.renderTrustPrompt()
	}

	return m.renderToolsList()
}

// renderToolsList renders the main tools list view
func (m Model) renderToolsList() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Padding(1, 2)

	contentStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Height(m.height - 6) // Reserve space for title, instructions, and message

	instructionsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(1, 2)

	var content strings.Builder

	if len(m.tools) == 0 {
		content.WriteString("No tools available.\n\n")
		content.WriteString("Tools will appear here when:\n")
		content.WriteString("• MCP servers are configured and connected\n")
		content.WriteString("• Built-in tools are registered\n\n")
		content.WriteString("Press 'r' to refresh the tools list.")
	} else {
		// Calculate visible range for scrolling
		visibleHeight := m.height - 8 // Account for UI chrome
		startIdx := 0
		endIdx := len(m.tools)

		if visibleHeight > 0 && len(m.tools) > visibleHeight {
			if m.selectedIndex >= m.scrollOffset+visibleHeight {
				m.scrollOffset = m.selectedIndex - visibleHeight + 1
			}
			if m.selectedIndex < m.scrollOffset {
				m.scrollOffset = m.selectedIndex
			}
			startIdx = m.scrollOffset
			endIdx = min(m.scrollOffset+visibleHeight, len(m.tools))
		}

		for i := startIdx; i < endIdx; i++ {
			tool := m.tools[i]
			prefix := "  "
			style := lipgloss.NewStyle()

			if i == m.selectedIndex {
				prefix = "▶ "
				style = style.
					Bold(true).
					Foreground(lipgloss.Color("15")).
					Background(lipgloss.Color("8"))
			}

			line := prefix + tool.Name + " (" + tool.Source + ") [" + tool.Trust.String() + "]"
			content.WriteString(style.Render(line) + "\n")

			if i == m.selectedIndex && tool.Description != "" {
				desc := "    " + tool.Description
				if len(desc) > m.width-6 {
					desc = desc[:m.width-9] + "..."
				}
				content.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("8")).
					Render(desc) + "\n")
			}
		}

		if len(m.tools) > visibleHeight && visibleHeight > 0 {
			scrollInfo := lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Render("Showing " + strconv.Itoa(startIdx+1) + "-" + strconv.Itoa(endIdx) + " of " + strconv.Itoa(len(m.tools)))
			content.WriteString("\n" + scrollInfo)
		}
	}

	instructions := "Navigation: ↑/↓ or j/k • Enter: Configure Trust • d: Details • r: Refresh • Tab: Switch tabs"

	var sections []string
	sections = append(sections, titleStyle.Render("🔧 Tools"))
	sections = append(sections, contentStyle.Render(content.String()))
	sections = append(sections, instructionsStyle.Render(instructions))

	if m.message != "" {
		sections = append(sections, m.messageStyle.Render(m.message))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderTrustPrompt renders the trust configuration prompt
func (m Model) renderTrustPrompt() string {
	if m.trustPrompt == nil {
		return "Error: No trust prompt"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Padding(1, 2)

	contentStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8"))

	instructionsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(1, 2)

	var content strings.Builder
	content.WriteString("Tool: " + m.trustPrompt.Tool.Name + "\n")
	content.WriteString("Description: " + m.trustPrompt.Tool.Description + "\n\n")
	content.WriteString("Select trust level:\n\n")

	trustLevels := []TrustLevel{TrustNone, TrustOnce, TrustSession}
	descriptions := []string{
		"Prompt for permission on each use",
		"Allow once, then revert to prompting",
		"Trust for entire session",
	}

	for i, level := range trustLevels {
		prefix := "  "
		style := lipgloss.NewStyle()

		if level == m.trustPrompt.SelectedLevel {
			prefix = "▶ "
			style = style.
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("8"))
		}

		line := prefix + level.String() + " - " + descriptions[i]
		content.WriteString(style.Render(line) + "\n")
	}

	instructions := "↑/↓ or j/k: Navigate • Enter: Apply • Escape: Cancel"

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("🔧 Configure Tool Trust"),
		contentStyle.Render(content.String()),
		instructionsStyle.Render(instructions),
	)
}

// AddTool adds a new tool to the list
func (m *Model) AddTool(tool Tool) {
	m.tools = append(m.tools, tool)
}

// GetTool returns the tool at the specified index
func (m Model) GetTool(index int) *Tool {
	if index < 0 || index >= len(m.tools) {
		return nil
	}
	return &m.tools[index]
}

// GetTools returns all tools
func (m Model) GetTools() []Tool {
	return m.tools
}

// UpdateToolTrust updates the trust level for a specific tool
func (m *Model) UpdateToolTrust(name string, trust TrustLevel) {
	for i := range m.tools {
		if m.tools[i].Name == name {
			m.tools[i].Trust = trust
			break
		}
	}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
