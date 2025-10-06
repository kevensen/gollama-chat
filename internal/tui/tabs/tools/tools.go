package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ollama/ollama/api"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
	"github.com/kevensen/gollama-chat/internal/tooling"
	"github.com/kevensen/gollama-chat/internal/tooling/mcp"
)

// TrustLevel represents the level of trust for a tool
type TrustLevel int

const (
	TrustNone    TrustLevel = iota // Not trusted, will block tool execution
	AskForTrust                    // Ask user for permission before each invocation
	TrustSession                   // Trusted for the entire session
)

func (tl TrustLevel) String() string {
	switch tl {
	case TrustNone:
		return "None"
	case AskForTrust:
		return "Ask"
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
	Source      string     `json:"source"`      // "builtin", "mcp", etc.
	ServerName  string     `json:"server_name"` // For MCP tools, the server name
	Available   bool       `json:"available"`   // Whether the tool is currently available
	APITool     *api.Tool  `json:"-"`           // The actual Ollama API tool
}

// Model represents the tools tab model
type Model struct {
	config        *configuration.Config
	ctx           context.Context
	mcpManager    *mcp.Manager
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
func NewModel(ctx context.Context, config *configuration.Config, mcpManager *mcp.Manager) Model {
	return Model{
		config:        config,
		ctx:           ctx,
		mcpManager:    mcpManager,
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
		logger := logging.WithComponent("tools-tab")
		logger.Debug("Received ToolsRefreshedMsg", "toolCount", len(msg.Tools), "hasError", msg.Error != nil)

		m.tools = msg.Tools
		if msg.Error != nil {
			m.message = "Error refreshing tools: " + msg.Error.Error()
			logger.Error("Tools refresh error", "error", msg.Error)
		} else {
			m.message = "Tools refreshed successfully"
			logger.Info("Tools refreshed in UI", "toolCount", len(m.tools))
		}
		if len(m.tools) > 0 && m.selectedIndex >= len(m.tools) {
			m.selectedIndex = len(m.tools) - 1
		}

		// Log each tool for debugging
		for i, tool := range m.tools {
			logger.Debug("Tool in UI list", "index", i, "name", tool.Name, "source", tool.Source, "available", tool.Available)
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
			// Save the trust level to configuration
			if err := m.config.SetToolTrustLevel(m.tools[m.selectedIndex].Name, int(m.trustPrompt.SelectedLevel)); err != nil {
				m.message = "Failed to save trust level: " + err.Error()
			} else {
				m.message = "Trust level updated for " + m.tools[m.selectedIndex].Name
			}
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
		logger := logging.WithComponent("tools-tab")
		logger.Info("Refreshing tools list for UI")

		var tools []Tool

		// Get built-in tools from the registry
		logger.Debug("Getting built-in tools from registry")
		builtinTools := tooling.DefaultRegistry.GetAllTools()
		logger.Info("Found built-in tools", "count", len(builtinTools))

		for _, builtinTool := range builtinTools {
			logger.Debug("Processing built-in tool", "name", builtinTool.Name(), "description", builtinTool.Description())

			// Load trust level from configuration, with fallback to defaults
			var trustLevel TrustLevel
			if m.config.ToolTrustLevels != nil {
				if configuredTrustLevel, exists := m.config.ToolTrustLevels[builtinTool.Name()]; exists {
					// Tool is explicitly configured
					trustLevel = TrustLevel(configuredTrustLevel)
				} else {
					// Not configured yet, use defaults
					defaultTrust := TrustNone
					if builtinTool.Name() == "filesystem_read" {
						defaultTrust = TrustSession // Filesystem read tool is trusted by default
					}
					trustLevel = defaultTrust
					// Save the default to configuration (ignore error during initial setup)
					_ = m.config.SetToolTrustLevel(builtinTool.Name(), int(defaultTrust))
				}
			} else {
				// No configuration map, use defaults
				defaultTrust := TrustNone
				if builtinTool.Name() == "filesystem_read" {
					defaultTrust = TrustSession // Filesystem read tool is trusted by default
				}
				trustLevel = defaultTrust
				// Save the default to configuration (ignore error during initial setup)
				_ = m.config.SetToolTrustLevel(builtinTool.Name(), int(defaultTrust))
			}

			tool := Tool{
				Name:        builtinTool.Name(),
				Description: builtinTool.Description(),
				Trust:       trustLevel,
				Source:      "builtin",
				ServerName:  "",
				Available:   true,
				UsageCount:  0,
				APITool:     builtinTool.GetAPITool(),
			}
			tools = append(tools, tool)
			logger.Debug("Added built-in tool to list", "name", tool.Name, "trust", tool.Trust.String())
		}

		// Get MCP tools from all servers (now with timeout protection in manager)
		logger.Debug("Getting MCP tools from servers")
		serverTools := m.mcpManager.GetAllTools()
		serverStatuses := m.mcpManager.GetAllServerStatuses()

		mcpToolCount := 0
		for serverName, mcpTools := range serverTools {
			serverStatus := serverStatuses[serverName]
			available := serverStatus == mcp.StatusRunning

			logger.Debug("Processing MCP server tools", "server", serverName, "status", serverStatus.String(), "available", available, "toolCount", len(mcpTools))

			for _, mcpTool := range mcpTools {
				mcpToolCount++
				// Create namespaced tool name
				fullName := serverName + "." + mcpTool.Name

				logger.Debug("Processing MCP tool", "server", serverName, "tool", mcpTool.Name, "fullName", fullName, "available", available)

				// Load trust level from configuration
				var trustLevel TrustLevel
				if m.config.ToolTrustLevels != nil {
					if configuredTrustLevel, exists := m.config.ToolTrustLevels[fullName]; exists {
						trustLevel = TrustLevel(configuredTrustLevel)
					} else {
						// Default to ask for MCP tools
						trustLevel = AskForTrust
						_ = m.config.SetToolTrustLevel(fullName, int(AskForTrust))
					}
				} else {
					trustLevel = AskForTrust
					_ = m.config.SetToolTrustLevel(fullName, int(AskForTrust))
				}

				// Convert MCP tool to Ollama API tool format
				apiTool := &api.Tool{
					Type: "function",
					Function: api.ToolFunction{
						Name:        fullName,
						Description: mcpTool.Description,
						Parameters:  convertMCPSchemaToOllamaParams(mcpTool.InputSchema),
					},
				}

				tool := Tool{
					Name:        fullName,
					Description: mcpTool.Description,
					Trust:       trustLevel,
					Source:      "mcp",
					ServerName:  serverName,
					Available:   available,
					UsageCount:  0,
					APITool:     apiTool,
				}
				tools = append(tools, tool)
				logger.Debug("Added MCP tool to list", "server", serverName, "tool", mcpTool.Name, "fullName", fullName, "trust", trustLevel.String(), "available", available)
			}
		}

		logger.Info("Tool refresh completed", "totalTools", len(tools), "builtinTools", len(builtinTools), "mcpTools", mcpToolCount)

		return ToolsRefreshedMsg{
			Tools: tools,
			Error: nil,
		}
	})
}

// convertMCPSchemaToOllamaParams converts MCP tool schema to Ollama parameters format
func convertMCPSchemaToOllamaParams(schema mcp.ToolSchema) api.ToolFunctionParameters {
	params := api.ToolFunctionParameters{
		Type:       schema.Type,
		Properties: make(map[string]api.ToolProperty),
		Required:   schema.Required,
	}

	for propName, propSchema := range schema.Properties {
		if propMap, ok := propSchema.(map[string]any); ok {
			property := api.ToolProperty{}

			if propType, exists := propMap["type"]; exists {
				if typeStr, ok := propType.(string); ok {
					property.Type = api.PropertyType{typeStr}
				}
			}

			if description, exists := propMap["description"]; exists {
				if descStr, ok := description.(string); ok {
					property.Description = descStr
				}
			}

			params.Properties[propName] = property
		}
	}

	return params
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
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8A7FD8")).
		Height(m.height - 8) // Reserve space for title, instructions, message, and border

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
		// Group tools by source
		builtinTools := []Tool{}
		mcpToolsByServer := make(map[string][]Tool)

		for _, tool := range m.tools {
			if tool.Source == "builtin" {
				builtinTools = append(builtinTools, tool)
			} else if tool.Source == "mcp" {
				if mcpToolsByServer[tool.ServerName] == nil {
					mcpToolsByServer[tool.ServerName] = []Tool{}
				}
				mcpToolsByServer[tool.ServerName] = append(mcpToolsByServer[tool.ServerName], tool)
			}
		}

		// Render built-in tools
		if len(builtinTools) > 0 {
			content.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("6")).
				Render("Built-in Tools"))
			content.WriteString("\n")

			for _, tool := range builtinTools {
				content.WriteString(m.renderTool(tool, m.getToolIndex(tool)))
				content.WriteString("\n")
			}
			content.WriteString("\n")
		}

		// Render MCP tools grouped by server
		for serverName, tools := range mcpToolsByServer {
			// Server header with status
			serverStatus := m.mcpManager.GetServerStatus(serverName)
			statusColor := "241" // Gray
			statusText := "●"
			switch serverStatus {
			case mcp.StatusRunning:
				statusColor = "46" // Green
			case mcp.StatusStarting:
				statusColor = "226" // Yellow
			case mcp.StatusError:
				statusColor = "196" // Red
			}

			statusIndicator := lipgloss.NewStyle().
				Foreground(lipgloss.Color(statusColor)).
				Render(statusText)

			serverHeader := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("5")).
				Render(fmt.Sprintf("MCP Server: %s %s (%s)",
					statusIndicator, serverName, serverStatus.String()))

			content.WriteString(serverHeader)
			content.WriteString("\n")

			// Render tools for this server
			if serverStatus != mcp.StatusRunning {
				// Show grayed out tools if server is not running
				grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
				content.WriteString(grayStyle.Render("  Server not running - tools unavailable"))
				content.WriteString("\n")
			} else {
				for _, tool := range tools {
					content.WriteString(m.renderTool(tool, m.getToolIndex(tool)))
					content.WriteString("\n")
				}
			}
			content.WriteString("\n")
		}
	}

	// Render title, instructions, content, and message
	title := titleStyle.Render("Tools")
	instructions := instructionsStyle.Render("↑/↓: navigate • Enter: toggle trust level • r: refresh • Ctrl+C: quit")

	var messageSection string
	if m.message != "" {
		messageSection = "\n" + m.messageStyle.Render(m.message)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		instructions,
		contentStyle.Render(content.String()),
		messageSection,
	)
}

// renderTool renders a single tool with proper styling based on selection and availability
func (m Model) renderTool(tool Tool, index int) string {
	prefix := "  "
	style := lipgloss.NewStyle()

	if index == m.selectedIndex {
		prefix = "▶ "
		style = style.
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("8"))
	}

	// Add availability indicator for MCP tools
	availabilityIndicator := ""
	if tool.Source == "mcp" {
		if tool.Available {
			availabilityIndicator = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46")).
				Render("●") + " "
		} else {
			availabilityIndicator = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Render("●") + " "
			// Gray out unavailable tools
			style = style.Foreground(lipgloss.Color("241"))
		}
	}

	// Format tool name (remove server prefix for display)
	displayName := tool.Name
	if tool.Source == "mcp" && strings.Contains(tool.Name, ".") {
		parts := strings.SplitN(tool.Name, ".", 2)
		if len(parts) == 2 {
			displayName = parts[1] // Show just the tool name, not server.tool
		}
	}

	line := prefix + availabilityIndicator + displayName +
		" [" + tool.Trust.String() + "]"

	if tool.Description != "" {
		maxDescLen := 50
		desc := tool.Description
		if len(desc) > maxDescLen {
			desc = desc[:maxDescLen-3] + "..."
		}
		line += " - " + desc
	}

	return style.Render(line)
}

// getToolIndex returns the index of a tool in the tools list
func (m Model) getToolIndex(targetTool Tool) int {
	for i, tool := range m.tools {
		if tool.Name == targetTool.Name && tool.Source == targetTool.Source {
			return i
		}
	}
	return -1
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
		BorderForeground(lipgloss.Color("#8A7FD8"))

	instructionsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(1, 2)

	var content strings.Builder
	content.WriteString("Tool: " + m.trustPrompt.Tool.Name + "\n")
	content.WriteString("Description: " + m.trustPrompt.Tool.Description + "\n\n")
	content.WriteString("Select trust level:\n\n")

	trustLevels := []TrustLevel{TrustNone, AskForTrust, TrustSession}
	descriptions := []string{
		"Block tool execution entirely",
		"Ask for permission before each use",
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
	// Update the tool in the model if it exists
	for i := range m.tools {
		if m.tools[i].Name == name {
			m.tools[i].Trust = trust
			break
		}
	}
	// Always save the trust level to configuration
	_ = m.config.SetToolTrustLevel(name, int(trust))
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
