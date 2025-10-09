package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
	mcpManager "github.com/kevensen/gollama-chat/internal/tooling/mcp"
)

// ServerStatus represents the UI status of an MCP server
type ServerUIStatus struct {
	Name      string
	Command   string
	Arguments []string
	Enabled   bool
	Status    mcpManager.ServerStatus
	LastError error
}

// Model represents the MCP tab model
type Model struct {
	config        *configuration.Config
	manager       *mcpManager.Manager
	servers       []ServerUIStatus
	selectedIndex int
	editingIndex  int // -1 when not editing, >= 0 when editing a server
	editingField  int // 0=name, 1=command, 2=arguments, 3=enabled
	inputValue    string
	showAddForm   bool
	newServer     configuration.MCPServer
	width         int
	height        int
	ctx           context.Context
	lastError     error // Last error encountered for display
}

// NewModel creates a new MCP tab model
func NewModel(ctx context.Context, config *configuration.Config, manager *mcpManager.Manager) Model {
	return Model{
		config:        config,
		manager:       manager,
		selectedIndex: 0,
		editingIndex:  -1,
		editingField:  0,
		showAddForm:   false,
		newServer: configuration.MCPServer{
			Name:      "",
			Command:   "",
			Arguments: []string{},
			Enabled:   true,
		},
		ctx: ctx,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	logger := logging.WithComponent("mcp-tab")
	logger.Info("Initializing MCP tab")
	return m.refreshServerList()
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.showAddForm {
			return m.handleAddFormKeys(msg)
		}

		if m.editingIndex >= 0 {
			return m.handleEditingKeys(msg)
		}

		return m.handleNormalKeys(msg)

	case refreshServersMsg:
		logger := logging.WithComponent("mcp-tab")
		logger.Info("Received refresh servers message", "serverCount", len(msg.servers))
		m.servers = msg.servers
		return m, nil

	case serverActionCompleteMsg:
		logger := logging.WithComponent("mcp-tab")
		if msg.err != nil {
			// Store error for display
			logger.Error("Server action failed", "error", msg.err)
			m.lastError = msg.err
			return m, m.refreshServerList()
		}
		// Reset form after successful save
		logger.Debug("Server action completed successfully, refreshing server list")
		m.lastError = nil
		m.showAddForm = false
		m.editingField = 0
		m.inputValue = ""
		m.newServer = configuration.MCPServer{
			Name:      "",
			Command:   "",
			Arguments: []string{},
			Enabled:   true,
		}
		// Use delayed refresh to allow time for async server operations to complete
		return m, tea.Batch(m.refreshServerList(), m.delayedRefresh())

	case delayedRefreshMsg:
		// Handle delayed refresh trigger
		return m, m.refreshServerList()
	}

	return m, nil
}

// handleNormalKeys handles keyboard input in normal mode
func (m Model) handleNormalKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
		return m, nil

	case "down", "j":
		if m.selectedIndex < len(m.servers)-1 {
			m.selectedIndex++
		}
		return m, nil

	case "enter":
		// Toggle server enabled/disabled
		if m.selectedIndex < len(m.servers) {
			return m, m.toggleServer(m.selectedIndex)
		}
		return m, nil

	case "e":
		// Edit server
		if m.selectedIndex < len(m.servers) {
			m.editingIndex = m.selectedIndex
			m.editingField = 0
			m.inputValue = m.servers[m.selectedIndex].Name
		}
		return m, nil

	case "d":
		// Delete server
		if m.selectedIndex < len(m.servers) {
			return m, m.deleteServer(m.selectedIndex)
		}
		return m, nil

	case "a":
		// Add new server
		m.showAddForm = true
		m.newServer = configuration.MCPServer{
			Name:      "",
			Command:   "",
			Arguments: []string{},
			Enabled:   true,
		}
		m.inputValue = ""
		m.editingField = 0
		return m, nil

	case "r":
		// Refresh server statuses
		return m, m.refreshServerList()

	case "c":
		// Clear error
		m.lastError = nil
		return m, nil
	}

	return m, nil
}

// handleAddFormKeys handles keyboard input while adding a new server
func (m Model) handleAddFormKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "escape", "esc":
		m.showAddForm = false
		m.editingField = 0
		m.inputValue = ""
		m.newServer = configuration.MCPServer{
			Name:      "",
			Command:   "",
			Arguments: []string{},
			Enabled:   true,
		}
		return m, nil

	case "enter":
		switch m.editingField {
		case 0: // Name
			m.newServer.Name = strings.TrimSpace(m.inputValue)
			m.editingField = 1
			m.inputValue = m.newServer.Command
		case 1: // Command
			m.newServer.Command = strings.TrimSpace(m.inputValue)
			m.editingField = 2
			m.inputValue = strings.Join(m.newServer.Arguments, " ")
		case 2: // Arguments
			args := strings.Fields(strings.TrimSpace(m.inputValue))
			m.newServer.Arguments = args
			m.editingField = 3
			m.inputValue = ""
		case 3: // Enabled (save)
			if m.newServer.Name != "" && m.newServer.Command != "" {
				m.showAddForm = false
				return m, m.addServer(m.newServer)
			}
		}
		return m, nil

	case "tab":
		// Save current field before moving to next
		switch m.editingField {
		case 0: // Name
			m.newServer.Name = strings.TrimSpace(m.inputValue)
		case 1: // Command
			m.newServer.Command = strings.TrimSpace(m.inputValue)
		case 2: // Arguments
			args := strings.Fields(strings.TrimSpace(m.inputValue))
			m.newServer.Arguments = args
		}

		// Move to next field
		m.editingField = (m.editingField + 1) % 4
		switch m.editingField {
		case 0:
			m.inputValue = m.newServer.Name
		case 1:
			m.inputValue = m.newServer.Command
		case 2:
			m.inputValue = strings.Join(m.newServer.Arguments, " ")
		case 3:
			m.inputValue = ""
		}
		return m, nil

	case "shift+tab":
		// Save current field before moving to previous
		switch m.editingField {
		case 0: // Name
			m.newServer.Name = strings.TrimSpace(m.inputValue)
		case 1: // Command
			m.newServer.Command = strings.TrimSpace(m.inputValue)
		case 2: // Arguments
			args := strings.Fields(strings.TrimSpace(m.inputValue))
			m.newServer.Arguments = args
		}

		// Move to previous field
		m.editingField = (m.editingField - 1 + 4) % 4
		switch m.editingField {
		case 0:
			m.inputValue = m.newServer.Name
		case 1:
			m.inputValue = m.newServer.Command
		case 2:
			m.inputValue = strings.Join(m.newServer.Arguments, " ")
		case 3:
			m.inputValue = ""
		}
		return m, nil

	case "shift+enter":
		// Save current field before moving to previous
		switch m.editingField {
		case 0: // Name
			m.newServer.Name = strings.TrimSpace(m.inputValue)
		case 1: // Command
			m.newServer.Command = strings.TrimSpace(m.inputValue)
		case 2: // Arguments
			args := strings.Fields(strings.TrimSpace(m.inputValue))
			m.newServer.Arguments = args
		}

		// Move to previous field
		m.editingField = (m.editingField - 1 + 4) % 4
		switch m.editingField {
		case 0:
			m.inputValue = m.newServer.Name
		case 1:
			m.inputValue = m.newServer.Command
		case 2:
			m.inputValue = strings.Join(m.newServer.Arguments, " ")
		case 3:
			m.inputValue = ""
		}
		return m, nil

	case "space", " ":
		if m.editingField == 3 { // Toggle enabled
			m.newServer.Enabled = !m.newServer.Enabled
		} else {
			m.inputValue += " "
		}
		return m, nil

	case "backspace":
		if len(m.inputValue) > 0 {
			m.inputValue = m.inputValue[:len(m.inputValue)-1]
		}
		return m, nil

	default:
		if len(msg.String()) == 1 && m.editingField < 3 {
			m.inputValue += msg.String()
		}
		return m, nil
	}
}

// handleEditingKeys handles keyboard input while editing a server
func (m Model) handleEditingKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "escape", "esc":
		m.editingIndex = -1
		return m, nil

	case "enter":
		// Save the current field and move to next
		server := m.servers[m.editingIndex]
		updatedServer := configuration.MCPServer{
			Name:      server.Name,
			Command:   server.Command,
			Arguments: server.Arguments,
			Enabled:   server.Enabled,
		}

		switch m.editingField {
		case 0: // Name
			updatedServer.Name = strings.TrimSpace(m.inputValue)
			m.editingField = 1
			m.inputValue = updatedServer.Command
		case 1: // Command
			updatedServer.Command = strings.TrimSpace(m.inputValue)
			m.editingField = 2
			m.inputValue = strings.Join(updatedServer.Arguments, " ")
		case 2: // Arguments
			args := strings.Fields(strings.TrimSpace(m.inputValue))
			updatedServer.Arguments = args
			m.editingField = 3
			m.inputValue = ""
		case 3: // Save
			m.editingIndex = -1
			return m, m.updateServer(server.Name, updatedServer)
		}
		return m, nil

	case "tab":
		// Move to next field
		m.editingField = (m.editingField + 1) % 4
		server := m.servers[m.editingIndex]
		switch m.editingField {
		case 0:
			m.inputValue = server.Name
		case 1:
			m.inputValue = server.Command
		case 2:
			m.inputValue = strings.Join(server.Arguments, " ")
		case 3:
			m.inputValue = ""
		}
		return m, nil

	case "shift+tab":
		// Move to previous field
		m.editingField = (m.editingField - 1 + 4) % 4
		server := m.servers[m.editingIndex]
		switch m.editingField {
		case 0:
			m.inputValue = server.Name
		case 1:
			m.inputValue = server.Command
		case 2:
			m.inputValue = strings.Join(server.Arguments, " ")
		case 3:
			m.inputValue = ""
		}
		return m, nil

	case "shift+enter":
		// Move to previous field
		m.editingField = (m.editingField - 1 + 4) % 4
		server := m.servers[m.editingIndex]
		switch m.editingField {
		case 0:
			m.inputValue = server.Name
		case 1:
			m.inputValue = server.Command
		case 2:
			m.inputValue = strings.Join(server.Arguments, " ")
		case 3:
			m.inputValue = ""
		}
		return m, nil

	case "space":
		if m.editingField == 3 { // Toggle enabled
			server := &m.servers[m.editingIndex]
			server.Enabled = !server.Enabled
		} else {
			m.inputValue += " "
		}
		return m, nil

	case "backspace":
		if len(m.inputValue) > 0 {
			m.inputValue = m.inputValue[:len(m.inputValue)-1]
		}
		return m, nil

	default:
		if len(msg.String()) == 1 && m.editingField < 3 {
			m.inputValue += msg.String()
		}
		return m, nil
	}
}

// View renders the MCP tab
func (m Model) View() string {
	var s strings.Builder

	// Header
	s.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("MCP Servers"))
	s.WriteString("\n")

	// Instructions
	if m.editingIndex >= 0 {
		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("Editing server - Enter: save field/next, Tab: next field, Shift+Tab/Shift+Enter: previous field, Esc: cancel"))
	} else {
		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("Enter: toggle, e: edit, d: delete, a: add, r: refresh, c: clear error"))
	}
	s.WriteString("\n")

	// Error display
	if m.lastError != nil {
		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Render(fmt.Sprintf("Error: %s", m.lastError.Error())))
		s.WriteString("\n")
	}

	// Server list
	if len(m.servers) == 0 {
		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("No MCP servers configured. Press 'a' to add one."))
	} else {
		for i, server := range m.servers {
			s.WriteString(m.renderServer(i, server))
			s.WriteString("\n")
		}
	}

	// Calculate heights for layout
	tabBarHeight := 1
	footerHeight := 1
	totalContentHeight := m.height - tabBarHeight - footerHeight + 2 // Add small adjustment to fill remaining space

	var serverListContent string
	var addFormContent string

	if m.showAddForm {
		// When showing add form, split the space
		serverListHeight := totalContentHeight * 2 / 3             // 2/3 for server list
		addFormHeight := totalContentHeight - serverListHeight - 2 // Remaining space minus spacing

		if serverListHeight < 1 {
			serverListHeight = 1
		}
		if addFormHeight < 1 {
			addFormHeight = 1
		}

		// Server list container
		serverListStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8A7FD8")).
			Padding(1, 2).
			Width(m.width - 2).
			Height(serverListHeight)

		serverListContent = serverListStyle.Render(s.String())

		// Add form container with different color
		addFormStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FFA500")). // Orange border for add form
			Padding(1, 2).
			Width(m.width - 2).
			Height(addFormHeight)

		addFormContent = addFormStyle.Render(m.renderAddFormContent())

		return lipgloss.JoinVertical(lipgloss.Left, serverListContent, addFormContent)
	} else {
		// Normal view with just server list
		containerStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8A7FD8")).
			Padding(1, 2).
			Width(m.width - 2).
			Height(totalContentHeight)

		return containerStyle.Render(s.String())
	}
}

// renderServer renders a single server entry
func (m Model) renderServer(index int, server ServerUIStatus) string {
	var style lipgloss.Style

	// Determine style based on selection and status
	if index == m.selectedIndex {
		style = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1)
	} else {
		style = lipgloss.NewStyle().Padding(0, 1)
	}

	// Status indicator
	statusColor := "241" // Gray
	statusText := "●"
	switch server.Status {
	case mcpManager.StatusRunning:
		statusColor = "46" // Green
	case mcpManager.StatusStarting:
		statusColor = "226" // Yellow
	case mcpManager.StatusError:
		statusColor = "196" // Red
	}

	statusIndicator := lipgloss.NewStyle().
		Foreground(lipgloss.Color(statusColor)).
		Render(statusText)

	// Enabled indicator
	enabledText := "✓"
	if !server.Enabled {
		enabledText = "✗"
	}

	enabledIndicator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")).
		Render(enabledText)
	if !server.Enabled {
		enabledIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(enabledText)
	}

	// If editing this server, show input fields
	if m.editingIndex == index {
		return m.renderEditingServer(server, style)
	}

	// Normal view
	name := server.Name
	if name == "" {
		name = "<unnamed>"
	}

	args := strings.Join(server.Arguments, " ")
	if len(args) > 50 {
		args = args[:47] + "..."
	}

	content := fmt.Sprintf("%s %s %s | %s %s",
		statusIndicator,
		enabledIndicator,
		name,
		server.Command,
		args)

	// Add error info if there's an error
	if server.LastError != nil {
		content += fmt.Sprintf(" | Error: %s", server.LastError.Error())
	}

	return style.Render(content)
}

// renderEditingServer renders a server in editing mode
func (m Model) renderEditingServer(server ServerUIStatus, style lipgloss.Style) string {
	fields := []string{
		fmt.Sprintf("Name: %s", m.getFieldValue(0, server.Name)),
		fmt.Sprintf("Command: %s", m.getFieldValue(1, server.Command)),
		fmt.Sprintf("Arguments: %s", m.getFieldValue(2, strings.Join(server.Arguments, " "))),
		fmt.Sprintf("Enabled: %v", server.Enabled),
	}

	// Highlight the current field
	if m.editingField < len(fields) {
		fields[m.editingField] = lipgloss.NewStyle().
			Background(lipgloss.Color("220")).
			Foreground(lipgloss.Color("0")).
			Render(fields[m.editingField])
	}

	return style.Render(strings.Join(fields, " | "))
}

// getFieldValue returns the appropriate value for a field (input or original)
func (m Model) getFieldValue(fieldIndex int, originalValue string) string {
	if m.editingField == fieldIndex {
		return m.inputValue + "█" // Cursor
	}
	return originalValue
}

// renderAddForm renders the add server form
func (m Model) renderAddForm() string {
	var s strings.Builder

	s.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("Add New MCP Server"))
	s.WriteString("\n\n")

	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Tab/Enter: next field, Shift+Tab/Shift+Enter: previous field, Space: toggle enabled, Esc: cancel"))
	s.WriteString("\n\n")

	// Form fields
	fields := []struct {
		label string
		value string
	}{
		{"Name", m.getAddFormValue(0, m.newServer.Name)},
		{"Command", m.getAddFormValue(1, m.newServer.Command)},
		{"Arguments", m.getAddFormValue(2, strings.Join(m.newServer.Arguments, " "))},
		{"Enabled", m.getEnabledFieldDisplay()},
	}

	for i, field := range fields {
		style := lipgloss.NewStyle()
		if i == m.editingField {
			style = style.Background(lipgloss.Color("220")).Foreground(lipgloss.Color("0"))
		}

		s.WriteString(fmt.Sprintf("%s: %s\n", field.label, style.Render(field.value)))
	}

	s.WriteString("\nPress Enter on Enabled field to save server.")

	return s.String()
}

// renderAddFormContent renders just the add form content without header/styling
func (m Model) renderAddFormContent() string {
	var s strings.Builder

	s.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("Add New MCP Server"))
	s.WriteString("\n\n")

	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Tab/Enter: next field, Shift+Tab/Shift+Enter: previous field, Space: toggle enabled, Esc: cancel"))
	s.WriteString("\n\n")

	// Form fields
	fields := []struct {
		label string
		value string
	}{
		{"Name", m.getAddFormValue(0, m.newServer.Name)},
		{"Command", m.getAddFormValue(1, m.newServer.Command)},
		{"Arguments", m.getAddFormValue(2, strings.Join(m.newServer.Arguments, " "))},
		{"Enabled", m.getEnabledFieldDisplay()},
	}

	for i, field := range fields {
		style := lipgloss.NewStyle()
		if i == m.editingField {
			style = style.Background(lipgloss.Color("220")).Foreground(lipgloss.Color("0"))
		}

		s.WriteString(fmt.Sprintf("%s: %s\n", field.label, style.Render(field.value)))
	}

	s.WriteString("\nPress Enter on Enabled field to save server.")

	return s.String()
}

// getAddFormValue returns the appropriate value for add form field
func (m Model) getAddFormValue(fieldIndex int, originalValue string) string {
	if m.editingField == fieldIndex && fieldIndex < 3 {
		return m.inputValue + "█" // Cursor
	}
	return originalValue
}

// getEnabledFieldDisplay returns the formatted enabled field display
func (m Model) getEnabledFieldDisplay() string {
	status := "✓ Enabled"
	if !m.newServer.Enabled {
		status = "✗ Disabled"
	}
	return fmt.Sprintf("%s (space to toggle)", status)
}

// Message types
type refreshServersMsg struct {
	servers []ServerUIStatus
}

type serverActionCompleteMsg struct {
	err error
}

// Commands
func (m Model) refreshServerList() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		logger := logging.WithComponent("mcp-tab")
		logger.Info("Refreshing server list", "configServerCount", len(m.config.MCPServers))

		var servers []ServerUIStatus

		// Get server statuses from manager
		statuses := m.manager.GetAllServerStatuses()
		logger.Info("Got statuses from manager", "statusCount", len(statuses))

		// Build UI server list from configuration
		for _, configServer := range m.config.MCPServers {
			logger.Info("Processing config server", "name", configServer.Name, "enabled", configServer.Enabled)
			status, exists := statuses[configServer.Name]
			if !exists {
				status = mcpManager.StatusStopped
			}

			var lastError error
			if status == mcpManager.StatusError {
				lastError = m.manager.GetServerLastError(configServer.Name)
			}

			servers = append(servers, ServerUIStatus{
				Name:      configServer.Name,
				Command:   configServer.Command,
				Arguments: configServer.Arguments,
				Enabled:   configServer.Enabled,
				Status:    status,
				LastError: lastError,
			})
		}

		logger.Info("Built server UI list", "serverCount", len(servers))
		return refreshServersMsg{servers: servers}
	})
}

func (m Model) toggleServer(index int) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if index >= len(m.servers) {
			return serverActionCompleteMsg{err: fmt.Errorf("invalid server index")}
		}

		server := m.servers[index]
		updatedServer := configuration.MCPServer{
			Name:      server.Name,
			Command:   server.Command,
			Arguments: server.Arguments,
			Enabled:   !server.Enabled, // Toggle
		}

		err := m.config.UpdateMCPServer(server.Name, updatedServer)
		if err != nil {
			return serverActionCompleteMsg{err: err}
		}

		// Update the manager with the new configuration
		if updateErr := m.manager.UpdateConfiguration(m.ctx, m.config); updateErr != nil {
			return serverActionCompleteMsg{err: updateErr}
		}

		return serverActionCompleteMsg{err: nil}
	})
}

func (m Model) deleteServer(index int) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if index >= len(m.servers) {
			return serverActionCompleteMsg{err: fmt.Errorf("invalid server index")}
		}

		server := m.servers[index]
		err := m.config.RemoveMCPServer(server.Name)
		if err != nil {
			// If the server is not found, consider it already deleted (successful operation)
			if strings.Contains(err.Error(), "not found") {
				// Server already deleted, update manager and continue
				if updateErr := m.manager.UpdateConfiguration(m.ctx, m.config); updateErr != nil {
					return serverActionCompleteMsg{err: updateErr}
				}
				return serverActionCompleteMsg{err: nil}
			}
			return serverActionCompleteMsg{err: err}
		}

		// Update the manager with the new configuration
		if updateErr := m.manager.UpdateConfiguration(m.ctx, m.config); updateErr != nil {
			return serverActionCompleteMsg{err: updateErr}
		}

		return serverActionCompleteMsg{err: nil}
	})
}

func (m Model) addServer(server configuration.MCPServer) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		logger := logging.WithComponent("mcp-tab")
		logger.Debug("Adding MCP server", "name", server.Name, "command", server.Command, "enabled", server.Enabled)

		err := m.config.AddMCPServer(server)
		if err != nil {
			logger.Error("Failed to add MCP server to config", "name", server.Name, "error", err)
			return serverActionCompleteMsg{err: err}
		}

		logger.Debug("Successfully added MCP server to config", "name", server.Name)

		// Update the manager with the new configuration
		if updateErr := m.manager.UpdateConfiguration(m.ctx, m.config); updateErr != nil {
			logger.Error("Failed to update MCP manager configuration", "name", server.Name, "error", updateErr)
			return serverActionCompleteMsg{err: updateErr}
		}

		logger.Debug("Successfully updated MCP manager configuration", "name", server.Name)
		return serverActionCompleteMsg{err: nil}
	})
}

func (m Model) updateServer(oldName string, server configuration.MCPServer) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		err := m.config.UpdateMCPServer(oldName, server)
		if err != nil {
			return serverActionCompleteMsg{err: err}
		}

		// Update the manager with the new configuration
		if updateErr := m.manager.UpdateConfiguration(m.ctx, m.config); updateErr != nil {
			return serverActionCompleteMsg{err: updateErr}
		}

		return serverActionCompleteMsg{err: nil}
	})
}

// delayedRefresh refreshes the server list after a short delay
// This helps with synchronization issues where servers need time to stop
func (m Model) delayedRefresh() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(time.Time) tea.Msg {
		return delayedRefreshMsg{}
	})
}

// Add a new message type for delayed refresh
type delayedRefreshMsg struct{}

// IsInFormMode returns true if the MCP tab is currently in a form editing mode
// (either adding a new server or editing an existing one)
func (m Model) IsInFormMode() bool {
	return m.showAddForm || m.editingIndex >= 0
}
