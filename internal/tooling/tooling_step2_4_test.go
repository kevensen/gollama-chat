package tooling

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Step 2.4 Tests: SetMCPManager, GetUnifiedTool, GetAllUnifiedTools operations via channels
// Comprehensive table-driven and synchronization tests

// Since we can't easily mock the mcp.Manager struct, we'll test without MCP operations
// and focus on the channel synchronization and builtin tool operations

// Table-driven test for channel operations (without MCP components)
func TestToolRegistry_Step2_4_ChannelOperationsTableDriven(t *testing.T) {
	tests := []struct {
		name                 string
		enableGetTool        bool
		enableRegister       bool
		enableGetAllTools    bool
		enableSetMCPManager  bool
		enableGetUnified     bool
		enableGetAllUnified  bool
		numBuiltinTools      int
		expectedBuiltinCount int
		expectedUnifiedCount int
	}{
		{
			name:                 "AllChannelsEnabled",
			enableGetTool:        true,
			enableRegister:       true,
			enableGetAllTools:    true,
			enableSetMCPManager:  true,
			enableGetUnified:     true,
			enableGetAllUnified:  true,
			numBuiltinTools:      3,
			expectedBuiltinCount: 3,
			expectedUnifiedCount: 3, // Only builtin tools without MCP
		},
		{
			name:                 "PartialChannelsEnabled",
			enableGetTool:        true,
			enableRegister:       false,
			enableGetAllTools:    true,
			enableSetMCPManager:  false,
			enableGetUnified:     true,
			enableGetAllUnified:  false,
			numBuiltinTools:      2,
			expectedBuiltinCount: 2,
			expectedUnifiedCount: 2, // Only builtin tools without MCP
		},
		{
			name:                 "NoChannelsEnabled",
			enableGetTool:        false,
			enableRegister:       false,
			enableGetAllTools:    false,
			enableSetMCPManager:  false,
			enableGetUnified:     false,
			enableGetAllUnified:  false,
			numBuiltinTools:      2,
			expectedBuiltinCount: 2,
			expectedUnifiedCount: 2, // Only builtin tools without MCP
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()
			defer registry.DisableChannelOperations()

			// Enable channels based on test configuration
			if tt.enableGetTool {
				registry.EnableChannelForGetTool()
			}
			if tt.enableRegister {
				registry.EnableChannelForRegister()
			}
			if tt.enableGetAllTools {
				registry.EnableChannelForGetAllTools()
			}
			if tt.enableSetMCPManager {
				registry.EnableChannelForSetMCPManager()
			}
			if tt.enableGetUnified {
				registry.EnableChannelForGetUnifiedTool()
			}
			if tt.enableGetAllUnified {
				registry.EnableChannelForGetAllUnifiedTools()
			}

			// Give worker time to initialize if channels enabled
			if tt.enableGetTool || tt.enableRegister || tt.enableGetAllTools ||
				tt.enableSetMCPManager || tt.enableGetUnified || tt.enableGetAllUnified {
				time.Sleep(10 * time.Millisecond)
			}

			// Register builtin tools
			for i := 0; i < tt.numBuiltinTools; i++ {
				tool := &MockBuiltinTool{
					name:        fmt.Sprintf("builtin_tool_%d", i),
					description: fmt.Sprintf("Builtin tool %d", i),
				}
				registry.Register(tool)
			}

			// Give time for operations to complete
			time.Sleep(10 * time.Millisecond)

			// Test GetAllTools (builtin only)
			allBuiltinTools := registry.GetAllTools()
			if len(allBuiltinTools) != tt.expectedBuiltinCount {
				t.Errorf("Expected %d builtin tools, got %d", tt.expectedBuiltinCount, len(allBuiltinTools))
			}

			// Test individual GetTool
			if tt.numBuiltinTools > 0 {
				tool, found := registry.GetTool("builtin_tool_0")
				if !found {
					t.Error("Should find builtin_tool_0")
				}
				if tool == nil {
					t.Error("Tool should not be nil")
				} else if tool.Name() != "builtin_tool_0" {
					t.Errorf("Expected 'builtin_tool_0', got '%s'", tool.Name())
				}
			}

			// Test GetAllUnifiedTools (builtin + MCP)
			allUnifiedTools := registry.GetAllUnifiedTools()
			if len(allUnifiedTools) != tt.expectedUnifiedCount {
				t.Errorf("Expected %d unified tools, got %d", tt.expectedUnifiedCount, len(allUnifiedTools))
			}

			// Test individual GetUnifiedTool
			if tt.numBuiltinTools > 0 {
				unifiedTool, found := registry.GetUnifiedTool("builtin_tool_0")
				if !found {
					t.Error("Should find builtin_tool_0 as unified tool")
				}
				if unifiedTool == nil {
					t.Error("Unified tool should not be nil")
				} else if unifiedTool.Name != "builtin_tool_0" {
					t.Errorf("Expected 'builtin_tool_0', got '%s'", unifiedTool.Name)
				}
			}
		})
	}
}

// Synchronization test - concurrent operations
func TestToolRegistry_Step2_4_ConcurrentOperations(t *testing.T) {
	registry := NewToolRegistry()
	defer registry.DisableChannelOperations()

	// Enable all channel operations
	registry.EnableChannelForGetTool()
	registry.EnableChannelForRegister()
	registry.EnableChannelForGetAllTools()
	registry.EnableChannelForSetMCPManager()
	registry.EnableChannelForGetUnifiedTool()
	registry.EnableChannelForGetAllUnifiedTools()

	// Give worker time to initialize
	time.Sleep(10 * time.Millisecond)

	const numGoroutines = 10
	const numOperationsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start multiple goroutines performing different operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperationsPerGoroutine; j++ {
				switch j % 5 {
				case 0:
					// Register tools
					tool := &MockBuiltinTool{
						name:        fmt.Sprintf("concurrent_tool_%d_%d", id, j),
						description: fmt.Sprintf("Concurrent tool %d %d", id, j),
					}
					registry.Register(tool)

				case 1:
					// Get individual tool
					registry.GetTool(fmt.Sprintf("concurrent_tool_%d_%d", id, j-1))

				case 2:
					// Get all builtin tools
					registry.GetAllTools()

				case 3:
					// Get unified tool
					registry.GetUnifiedTool(fmt.Sprintf("concurrent_tool_%d_%d", id, j-1))

				case 4:
					// Get all unified tools
					registry.GetAllUnifiedTools()
				}

				// Small delay to encourage interleaving
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify final state
	allTools := registry.GetAllTools()
	allUnified := registry.GetAllUnifiedTools()

	t.Logf("Final state: %d builtin tools, %d unified tools", len(allTools), len(allUnified))

	// Should have tools from concurrent operations
	if len(allTools) == 0 {
		t.Error("Expected some tools to be registered")
	}
	if len(allUnified) == 0 {
		t.Error("Expected some unified tools to exist")
	}
}

// Race condition detection test
func TestToolRegistry_Step2_4_RaceConditionDetection(t *testing.T) {
	registry := NewToolRegistry()
	defer registry.DisableChannelOperations()

	// Enable all operations
	registry.EnableChannelForGetTool()
	registry.EnableChannelForRegister()
	registry.EnableChannelForGetAllTools()
	registry.EnableChannelForGetUnifiedTool()
	registry.EnableChannelForGetAllUnifiedTools()

	time.Sleep(10 * time.Millisecond)

	const numGoroutines = 50
	const numIterations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Readers and writers

	// Writer goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				tool := &MockBuiltinTool{
					name:        fmt.Sprintf("race_tool_%d_%d", id, j),
					description: fmt.Sprintf("Race test tool %d %d", id, j),
				}
				registry.Register(tool)
			}
		}(i)
	}

	// Reader goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				// Mix of read operations
				switch j % 4 {
				case 0:
					registry.GetTool(fmt.Sprintf("race_tool_%d_%d", id, j))
				case 1:
					registry.GetAllTools()
				case 2:
					registry.GetUnifiedTool(fmt.Sprintf("race_tool_%d_%d", id, j))
				case 3:
					registry.GetAllUnifiedTools()
				}
			}
		}(i)
	}

	wg.Wait()

	// If we reach here without race detector issues, the test passes
	t.Log("Race condition test completed successfully")
}

// Test channel shutdown behavior
func TestToolRegistry_Step2_4_ChannelShutdown(t *testing.T) {
	tests := []struct {
		name             string
		enableOperations []string
	}{
		{
			name:             "SingleOperation",
			enableOperations: []string{"GetTool"},
		},
		{
			name:             "MultipleOperations",
			enableOperations: []string{"GetTool", "Register", "GetAllTools"},
		},
		{
			name:             "AllOperations",
			enableOperations: []string{"GetTool", "Register", "GetAllTools", "SetMCPManager", "GetUnifiedTool", "GetAllUnifiedTools"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()

			// Enable specified operations
			for _, op := range tt.enableOperations {
				switch op {
				case "GetTool":
					registry.EnableChannelForGetTool()
				case "Register":
					registry.EnableChannelForRegister()
				case "GetAllTools":
					registry.EnableChannelForGetAllTools()
				case "SetMCPManager":
					registry.EnableChannelForSetMCPManager()
				case "GetUnifiedTool":
					registry.EnableChannelForGetUnifiedTool()
				case "GetAllUnifiedTools":
					registry.EnableChannelForGetAllUnifiedTools()
				}
			}

			// Give worker time to initialize
			time.Sleep(10 * time.Millisecond)

			// Register a test tool
			testTool := &MockBuiltinTool{
				name:        "shutdown_test_tool",
				description: "Tool for shutdown testing",
			}
			registry.Register(testTool)

			// Verify operations work before shutdown
			tool, found := registry.GetTool("shutdown_test_tool")
			if !found {
				t.Error("Should find tool before shutdown")
			}
			if tool == nil || tool.Name() != "shutdown_test_tool" {
				t.Error("Tool should be valid before shutdown")
			}

			// Shutdown channels
			registry.DisableChannelOperations()

			// Verify operations still work via mutex fallback
			tool, found = registry.GetTool("shutdown_test_tool")
			if !found {
				t.Error("Should find tool after shutdown (mutex fallback)")
			}
			if tool == nil || tool.Name() != "shutdown_test_tool" {
				t.Error("Tool should be valid after shutdown (mutex fallback)")
			}

			allTools := registry.GetAllTools()
			if len(allTools) == 0 {
				t.Error("Should have tools after shutdown (mutex fallback)")
			}
		})
	}
}

// Test SetMCPManager channel enablement
func TestToolRegistry_Step2_4_SetMCPManagerChannel(t *testing.T) {
	registry := NewToolRegistry()
	defer registry.DisableChannelOperations()

	// Test enabling SetMCPManager via channels
	registry.EnableChannelForSetMCPManager()
	time.Sleep(10 * time.Millisecond)

	// Test nil MCP manager (should handle gracefully)
	registry.SetMCPManager(nil)

	// Operations should still work
	allUnified := registry.GetAllUnifiedTools()
	t.Logf("Unified tools with nil MCP manager: %d", len(allUnified))
}
