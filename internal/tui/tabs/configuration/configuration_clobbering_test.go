package configuration

import (
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

// TestConfigClobberingFix tests that the configuration tab doesn't clobber MCP server settings
func TestConfigClobberingFix(t *testing.T) {
	// Create initial config with an MCP server
	initialConfig := configuration.DefaultConfig()
	mcpServer := configuration.MCPServer{
		Name:      "test-server",
		Command:   "/usr/bin/test",
		Arguments: []string{"--arg1", "value1"},
		Enabled:   true,
	}
	err := initialConfig.AddMCPServerNoSave(mcpServer)
	if err != nil {
		t.Fatalf("Failed to add MCP server: %v", err)
	}

	// Create configuration tab model
	model := NewModel(initialConfig)

	// Verify that editConfig has the MCP server
	if len(model.editConfig.MCPServers) != 1 {
		t.Errorf("Expected 1 MCP server in editConfig, got %d", len(model.editConfig.MCPServers))
	}

	// Simulate another tab adding a new MCP server to the main config
	newMCPServer := configuration.MCPServer{
		Name:      "another-server",
		Command:   "/usr/bin/another",
		Arguments: []string{"--arg2", "value2"},
		Enabled:   false,
	}
	err = initialConfig.AddMCPServerNoSave(newMCPServer)
	if err != nil {
		t.Fatalf("Failed to add second MCP server: %v", err)
	}

	// Now the main config has 2 servers, but editConfig still has 1
	if len(initialConfig.MCPServers) != 2 {
		t.Errorf("Expected 2 MCP servers in main config, got %d", len(initialConfig.MCPServers))
	}
	if len(model.editConfig.MCPServers) != 1 {
		t.Errorf("Expected 1 MCP server in editConfig before sync, got %d", len(model.editConfig.MCPServers))
	}

	// Test the sync function
	model.syncEditConfigWithMain()

	// After sync, editConfig should have both MCP servers
	if len(model.editConfig.MCPServers) != 2 {
		t.Errorf("Expected 2 MCP servers in editConfig after sync, got %d", len(model.editConfig.MCPServers))
	}

	// Verify the servers are correctly copied
	found1, found2 := false, false
	for _, server := range model.editConfig.MCPServers {
		if server.Name == "test-server" {
			found1 = true
			if server.Command != "/usr/bin/test" || len(server.Arguments) != 2 || !server.Enabled {
				t.Errorf("First MCP server data corrupted after sync: %+v", server)
			}
		}
		if server.Name == "another-server" {
			found2 = true
			if server.Command != "/usr/bin/another" || len(server.Arguments) != 2 || server.Enabled {
				t.Errorf("Second MCP server data corrupted after sync: %+v", server)
			}
		}
	}

	if !found1 {
		t.Error("First MCP server not found after sync")
	}
	if !found2 {
		t.Error("Second MCP server not found after sync")
	}
}

// TestEditConfigPreservesChanges tests that syncEditConfigWithMain preserves user edits
func TestEditConfigPreservesChanges(t *testing.T) {
	// Create initial config
	initialConfig := configuration.DefaultConfig()
	initialConfig.ChatModel = "original-model"
	initialConfig.OllamaURL = "http://original:11434"

	// Create configuration tab model
	model := NewModel(initialConfig)

	// Simulate user editing some fields
	model.editConfig.ChatModel = "edited-model"
	model.editConfig.OllamaURL = "http://edited:11434"

	// Simulate another tab making changes to main config
	mcpServer := configuration.MCPServer{
		Name:    "new-server",
		Command: "/usr/bin/new",
		Enabled: true,
	}
	err := initialConfig.AddMCPServerNoSave(mcpServer)
	if err != nil {
		t.Fatalf("Failed to add MCP server: %v", err)
	}

	// Sync editConfig with main config
	model.syncEditConfigWithMain()

	// Verify that user edits are preserved
	if model.editConfig.ChatModel != "edited-model" {
		t.Errorf("Expected edited ChatModel to be preserved, got %s", model.editConfig.ChatModel)
	}
	if model.editConfig.OllamaURL != "http://edited:11434" {
		t.Errorf("Expected edited OllamaURL to be preserved, got %s", model.editConfig.OllamaURL)
	}

	// Verify that MCP server changes are picked up
	if len(model.editConfig.MCPServers) != 1 {
		t.Errorf("Expected 1 MCP server after sync, got %d", len(model.editConfig.MCPServers))
	}
	if len(model.editConfig.MCPServers) > 0 && model.editConfig.MCPServers[0].Name != "new-server" {
		t.Errorf("Expected MCP server name 'new-server', got %s", model.editConfig.MCPServers[0].Name)
	}
}
