package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
)

// Manager handles multiple MCP server clients
type Manager struct {
	clients    map[string]*Client
	clientsMux sync.RWMutex
	config     *configuration.Config
}

// NewManager creates a new MCP manager
func NewManager(config *configuration.Config) *Manager {
	logger := logging.WithComponent("mcp-manager")
	logger.Debug("Creating new MCP manager")

	return &Manager{
		clients: make(map[string]*Client),
		config:  config,
	}
}

// StartEnabledServers starts all enabled MCP servers from the configuration
func (m *Manager) StartEnabledServers(ctx context.Context) error {
	logger := logging.WithComponent("mcp-manager")
	logger.Info("Starting enabled MCP servers")

	m.clientsMux.Lock()
	defer m.clientsMux.Unlock()

	enabledServers := m.config.GetEnabledMCPServers()
	logger.Debug("Found enabled servers", "count", len(enabledServers))

	var errors []error

	for _, server := range enabledServers {
		logger.Debug("Starting MCP server", "name", server.Name, "command", server.Command, "args", server.Arguments, "enabled", server.Enabled)
		client := NewClient(ctx, server)
		m.clients[server.Name] = client

		if err := client.Start(); err != nil {
			logger.Error("Failed to start MCP server", "name", server.Name, "error", err)
			errors = append(errors, fmt.Errorf("failed to start server %s: %w", server.Name, err))
		} else {
			logger.Info("Successfully started MCP server", "name", server.Name)
		}
	}

	if len(errors) > 0 {
		logger.Warn("Some MCP servers failed to start", "errorCount", len(errors))
		// Return the first error, but continue trying to start other servers
		return errors[0]
	}

	logger.Info("All enabled MCP servers started successfully")
	return nil
}

// StopAllServers stops all running MCP servers
func (m *Manager) StopAllServers() error {
	m.clientsMux.Lock()
	defer m.clientsMux.Unlock()

	var errors []error

	for name, client := range m.clients {
		if err := client.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop server %s: %w", name, err))
		}
	}

	// Clear the clients map
	m.clients = make(map[string]*Client)

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// GetServerStatus returns the status of a specific server
func (m *Manager) GetServerStatus(serverName string) ServerStatus {
	m.clientsMux.RLock()
	defer m.clientsMux.RUnlock()

	client, exists := m.clients[serverName]
	if !exists {
		return StatusStopped
	}

	return client.GetStatus()
}

// GetServerLastError returns the last error for a specific server
func (m *Manager) GetServerLastError(serverName string) error {
	m.clientsMux.RLock()
	defer m.clientsMux.RUnlock()

	client, exists := m.clients[serverName]
	if !exists {
		return fmt.Errorf("server %s is not running", serverName)
	}

	return client.GetLastError()
}

// GetAllServerStatuses returns the status of all configured servers
func (m *Manager) GetAllServerStatuses() map[string]ServerStatus {
	m.clientsMux.RLock()
	defer m.clientsMux.RUnlock()

	statuses := make(map[string]ServerStatus)

	// Add statuses for all configured servers
	for _, server := range m.config.MCPServers {
		if client, exists := m.clients[server.Name]; exists {
			// Use a timeout to prevent hanging on GetStatus calls
			statusChan := make(chan ServerStatus, 1)
			go func(c *Client) {
				status := c.GetStatus()
				select {
				case statusChan <- status:
				default:
					// Channel full, ignore
				}
			}(client)

			select {
			case status := <-statusChan:
				statuses[server.Name] = status
			case <-time.After(1 * time.Second):
				// If we can't get status quickly, assume stopped
				statuses[server.Name] = StatusStopped
			}
		} else {
			statuses[server.Name] = StatusStopped
		}
	}

	return statuses
}

// GetAllTools returns all tools from all running servers, namespaced by server name
func (m *Manager) GetAllTools() map[string][]Tool {
	logger := logging.WithComponent("mcp-manager")
	logger.Debug("Getting all tools from MCP servers")

	m.clientsMux.RLock()
	defer m.clientsMux.RUnlock()

	allTools := make(map[string][]Tool)
	totalTools := 0

	for serverName, client := range m.clients {
		// Use a timeout for the entire client interaction to prevent deadlocks
		clientChan := make(chan struct {
			tools []Tool
			valid bool
		}, 1)

		go func(c *Client, name string) {
			// Check client status first to avoid calling methods on stopped clients
			statusChan := make(chan ServerStatus, 1)
			go func() {
				statusChan <- c.GetStatus()
			}()

			var status ServerStatus
			select {
			case status = <-statusChan:
			case <-time.After(500 * time.Millisecond):
				// Status check timed out, assume stopped
				clientChan <- struct {
					tools []Tool
					valid bool
				}{nil, false}
				return
			}

			if status == StatusRunning {
				// Get tools with timeout
				toolsChan := make(chan []Tool, 1)
				go func() {
					toolsChan <- c.GetTools()
				}()

				select {
				case tools := <-toolsChan:
					clientChan <- struct {
						tools []Tool
						valid bool
					}{tools, true}
				case <-time.After(1 * time.Second):
					logger.Warn("Timeout while getting tools from MCP server", "server", name)
					clientChan <- struct {
						tools []Tool
						valid bool
					}{nil, false}
				}
			} else {
				logger.Debug("Skipping non-running MCP server", "server", name, "status", status.String())
				clientChan <- struct {
					tools []Tool
					valid bool
				}{nil, false}
			}
		}(client, serverName)

		// Wait for result with overall timeout
		select {
		case result := <-clientChan:
			if result.valid && len(result.tools) > 0 {
				allTools[serverName] = result.tools
				totalTools += len(result.tools)
				logger.Debug("Retrieved tools from MCP server", "server", serverName, "toolCount", len(result.tools))
			} else if result.valid {
				logger.Debug("No tools available from MCP server", "server", serverName)
			}
		case <-time.After(2 * time.Second):
			logger.Warn("Overall timeout while processing MCP server", "server", serverName)
		}
	}

	logger.Debug("Retrieved all tools from MCP servers", "serverCount", len(allTools), "totalTools", totalTools)
	return allTools
}

// CallTool calls a tool on a specific server
func (m *Manager) CallTool(serverName, toolName string, arguments map[string]any) (*CallToolResult, error) {
	m.clientsMux.RLock()
	client, exists := m.clients[serverName]
	m.clientsMux.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server %s is not running", serverName)
	}

	if client.GetStatus() != StatusRunning {
		return nil, fmt.Errorf("server %s is not running (status: %s)", serverName, client.GetStatus())
	}

	return client.CallTool(toolName, arguments)
}

// RefreshTools refreshes tools for all running servers
func (m *Manager) RefreshTools() error {
	logger := logging.WithComponent("mcp-manager")
	logger.Debug("Refreshing tools for all running MCP servers")

	m.clientsMux.RLock()
	defer m.clientsMux.RUnlock()

	var errors []error
	refreshedServers := 0

	for serverName, client := range m.clients {
		if client.GetStatus() == StatusRunning {
			logger.Debug("Refreshing tools for MCP server", "server", serverName)
			if err := client.refreshTools(); err != nil {
				logger.Error("Failed to refresh tools for MCP server", "server", serverName, "error", err)
				errors = append(errors, fmt.Errorf("failed to refresh tools for server %s: %w", serverName, err))
			} else {
				refreshedServers++
				logger.Debug("Successfully refreshed tools for MCP server", "server", serverName)
			}
		} else {
			logger.Debug("Skipping tool refresh for non-running MCP server", "server", serverName, "status", client.GetStatus().String())
		}
	}

	logger.Info("Tool refresh completed", "refreshedServers", refreshedServers, "totalServers", len(m.clients), "errors", len(errors))

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// UpdateConfiguration updates the manager with a new configuration
// This will start newly enabled servers and stop disabled ones
func (m *Manager) UpdateConfiguration(ctx context.Context, config *configuration.Config) error {
	m.config = config

	m.clientsMux.Lock()
	defer m.clientsMux.Unlock()

	// Get current running servers
	currentServers := make(map[string]bool)
	for name := range m.clients {
		currentServers[name] = true
	}

	// Get enabled servers from new config
	enabledServers := make(map[string]configuration.MCPServer)
	for _, server := range config.GetEnabledMCPServers() {
		enabledServers[server.Name] = server
	}

	// Stop servers that are no longer enabled or configured
	for name, client := range m.clients {
		if _, stillEnabled := enabledServers[name]; !stillEnabled {
			client.Stop()
			delete(m.clients, name)
		}
	}

	// Start newly enabled servers
	var errors []error
	for name, server := range enabledServers {
		if _, alreadyRunning := currentServers[name]; !alreadyRunning {
			client := NewClient(ctx, server)
			m.clients[name] = client
			if err := client.Start(); err != nil {
				errors = append(errors, fmt.Errorf("failed to start server %s: %w", name, err))
			}
		}
	}

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}
