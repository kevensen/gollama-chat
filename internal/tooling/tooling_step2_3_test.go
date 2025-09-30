package tooling

import (
	"testing"
	"time"
)

// Step 2.3 Tests: GetAllTools operation via channels
// Testing the channel-based GetAllTools operation individually and in integration

func TestToolRegistry_Step2_3_GetAllToolsViaChannels(t *testing.T) {
	registry := NewToolRegistry()

	// Test empty registry first
	tools := registry.GetAllTools()
	if len(tools) != 0 {
		t.Errorf("Expected empty registry, got %d tools", len(tools))
	}

	// Register some tools using mutex first
	mockTool1 := &MockBuiltinTool{name: "test_tool_1", description: "Test tool 1"}
	mockTool2 := &MockBuiltinTool{name: "test_tool_2", description: "Test tool 2"}

	registry.Register(mockTool1)
	registry.Register(mockTool2)

	// Verify mutex-based GetAllTools works
	tools = registry.GetAllTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools in registry, got %d", len(tools))
	}

	// Enable channel-based GetAllTools
	registry.EnableChannelForGetAllTools()

	// Give worker time to initialize
	time.Sleep(10 * time.Millisecond)

	// Test channel-based GetAllTools
	tools = registry.GetAllTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools via channels, got %d", len(tools))
	}

	// Verify we got the correct tools
	if _, exists := tools["test_tool_1"]; !exists {
		t.Error("test_tool_1 not found in channel-based GetAllTools")
	}
	if _, exists := tools["test_tool_2"]; !exists {
		t.Error("test_tool_2 not found in channel-based GetAllTools")
	}

	// Verify tool contents
	tool1 := tools["test_tool_1"]
	if tool1.Name() != "test_tool_1" {
		t.Errorf("Expected tool name 'test_tool_1', got '%s'", tool1.Name())
	}
	if tool1.Description() != "Test tool 1" {
		t.Errorf("Expected description 'Test tool 1', got '%s'", tool1.Description())
	}

	// Clean up
	registry.DisableChannelOperations()
}

func TestToolRegistry_Step2_3_GetAllToolsChannelAfterRegister(t *testing.T) {
	registry := NewToolRegistry()

	// Enable both Register and GetAllTools via channels
	registry.EnableChannelForRegister()
	registry.EnableChannelForGetAllTools()

	// Give worker time to initialize
	time.Sleep(10 * time.Millisecond)

	// Start with empty registry
	tools := registry.GetAllTools()
	if len(tools) != 0 {
		t.Errorf("Expected empty registry, got %d tools", len(tools))
	}

	// Register tools via channels
	mockTool1 := &MockBuiltinTool{name: "channel_tool_1", description: "Channel test tool 1"}
	mockTool2 := &MockBuiltinTool{name: "channel_tool_2", description: "Channel test tool 2"}

	registry.Register(mockTool1)
	registry.Register(mockTool2)

	// Give some time for channel operations to complete
	time.Sleep(10 * time.Millisecond)

	// Verify we can get all tools via channels
	tools = registry.GetAllTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools after channel registration, got %d", len(tools))
	}

	// Verify tool contents
	if _, exists := tools["channel_tool_1"]; !exists {
		t.Error("channel_tool_1 not found after channel registration")
	}
	if _, exists := tools["channel_tool_2"]; !exists {
		t.Error("channel_tool_2 not found after channel registration")
	}

	// Add one more tool and verify GetAllTools updates
	mockTool3 := &MockBuiltinTool{name: "channel_tool_3", description: "Channel test tool 3"}
	registry.Register(mockTool3)

	// Give some time for registration
	time.Sleep(10 * time.Millisecond)

	tools = registry.GetAllTools()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools after adding third tool, got %d", len(tools))
	}

	if _, exists := tools["channel_tool_3"]; !exists {
		t.Error("channel_tool_3 not found after adding third tool")
	}

	// Clean up
	registry.DisableChannelOperations()
}

func TestToolRegistry_Step2_3_IntegrationThreeOperations(t *testing.T) {
	registry := NewToolRegistry()

	// Enable all three channel operations: GetTool, Register, GetAllTools
	registry.EnableChannelForGetTool()
	registry.EnableChannelForRegister()
	registry.EnableChannelForGetAllTools()

	// Give worker time to initialize
	time.Sleep(10 * time.Millisecond)

	// Register some tools via channels
	mockTool1 := &MockBuiltinTool{name: "integration_tool_1", description: "Integration test tool 1"}
	mockTool2 := &MockBuiltinTool{name: "integration_tool_2", description: "Integration test tool 2"}

	registry.Register(mockTool1)
	registry.Register(mockTool2)

	// Give some time for operations
	time.Sleep(10 * time.Millisecond)

	// Test GetTool via channels
	tool, found := registry.GetTool("integration_tool_1")
	if !found {
		t.Error("integration_tool_1 not found via GetTool channel")
	}
	if tool == nil {
		t.Error("GetTool returned nil tool")
	} else if tool.Name() != "integration_tool_1" {
		t.Errorf("Expected tool name 'integration_tool_1', got '%s'", tool.Name())
	}

	// Test GetAllTools via channels
	allTools := registry.GetAllTools()
	if len(allTools) != 2 {
		t.Errorf("Expected 2 tools via GetAllTools channel, got %d", len(allTools))
	}

	// Verify both tools are present
	if _, exists := allTools["integration_tool_1"]; !exists {
		t.Error("integration_tool_1 not found in GetAllTools channel result")
	}
	if _, exists := allTools["integration_tool_2"]; !exists {
		t.Error("integration_tool_2 not found in GetAllTools channel result")
	}

	// Test that GetTool works for the second tool too
	tool2, found2 := registry.GetTool("integration_tool_2")
	if !found2 {
		t.Error("integration_tool_2 not found via GetTool channel")
	}
	if tool2 == nil {
		t.Error("GetTool returned nil for second tool")
	} else if tool2.Name() != "integration_tool_2" {
		t.Errorf("Expected tool name 'integration_tool_2', got '%s'", tool2.Name())
	}

	// Test non-existent tool
	_, found3 := registry.GetTool("nonexistent_tool")
	if found3 {
		t.Error("Expected false for non-existent tool, got true")
	}

	// Clean up
	registry.DisableChannelOperations()
}

func TestToolRegistry_Step2_3_GetAllToolsShutdown(t *testing.T) {
	registry := NewToolRegistry()

	// Register a tool first
	mockTool := &MockBuiltinTool{name: "shutdown_tool", description: "Tool for shutdown test"}
	registry.Register(mockTool)

	// Enable channel-based GetAllTools
	registry.EnableChannelForGetAllTools()

	// Give worker time to initialize
	time.Sleep(10 * time.Millisecond)

	// Verify GetAllTools works initially
	tools := registry.GetAllTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool before shutdown, got %d", len(tools))
	}

	// Shutdown channel operations
	registry.DisableChannelOperations()

	// Verify GetAllTools still works via mutex fallback
	tools = registry.GetAllTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after shutdown (mutex fallback), got %d", len(tools))
	}

	if _, exists := tools["shutdown_tool"]; !exists {
		t.Error("shutdown_tool not found after channel shutdown")
	}
}
