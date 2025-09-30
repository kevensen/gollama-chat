package tooling

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Benchmark tests comparing channel vs mutex performance
func BenchmarkToolRegistry_GetTool(b *testing.B) {
	tests := []struct {
		name          string
		useChannels   bool
		numTools      int
		numGoroutines int
	}{
		{"Mutex_1Tool_1Goroutine", false, 1, 1},
		{"Channel_1Tool_1Goroutine", true, 1, 1},
		{"Mutex_10Tools_1Goroutine", false, 10, 1},
		{"Channel_10Tools_1Goroutine", true, 10, 1},
		{"Mutex_10Tools_10Goroutines", false, 10, 10},
		{"Channel_10Tools_10Goroutines", true, 10, 10},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			registry := NewToolRegistry()
			defer registry.DisableChannelOperations()

			// Setup
			if tt.useChannels {
				registry.EnableChannelForGetTool()
				registry.EnableChannelForRegister()
				time.Sleep(10 * time.Millisecond) // Let worker initialize
			}

			// Register tools
			for i := 0; i < tt.numTools; i++ {
				tool := &MockBuiltinTool{
					name:        fmt.Sprintf("bench_tool_%d", i),
					description: fmt.Sprintf("Benchmark tool %d", i),
				}
				registry.Register(tool)
			}

			// Give time for registration to complete
			if tt.useChannels {
				time.Sleep(10 * time.Millisecond)
			}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				toolIndex := 0
				for pb.Next() {
					toolName := fmt.Sprintf("bench_tool_%d", toolIndex%tt.numTools)
					registry.GetTool(toolName)
					toolIndex++
				}
			})
		})
	}
}

func BenchmarkToolRegistry_GetAllTools(b *testing.B) {
	tests := []struct {
		name        string
		useChannels bool
		numTools    int
	}{
		{"Mutex_10Tools", false, 10},
		{"Channel_10Tools", true, 10},
		{"Mutex_100Tools", false, 100},
		{"Channel_100Tools", true, 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			registry := NewToolRegistry()
			defer registry.DisableChannelOperations()

			// Setup
			if tt.useChannels {
				registry.EnableChannelForGetTool()
				registry.EnableChannelForRegister()
				registry.EnableChannelForGetAllTools()
				time.Sleep(10 * time.Millisecond)
			}

			// Register tools
			for i := 0; i < tt.numTools; i++ {
				tool := &MockBuiltinTool{
					name:        fmt.Sprintf("bench_tool_%d", i),
					description: fmt.Sprintf("Benchmark tool %d", i),
				}
				registry.Register(tool)
			}

			if tt.useChannels {
				time.Sleep(10 * time.Millisecond)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				registry.GetAllTools()
			}
		})
	}
}

// Comprehensive test for all-channels-enabled scenario
func TestToolRegistry_AllChannelsEnabled_Comprehensive(t *testing.T) {
	registry := NewToolRegistry()
	defer registry.DisableChannelOperations()

	// Enable ALL channel operations
	registry.EnableChannelForGetTool()
	registry.EnableChannelForRegister()
	registry.EnableChannelForGetAllTools()
	registry.EnableChannelForSetMCPManager()
	registry.EnableChannelForGetUnifiedTool()
	registry.EnableChannelForGetAllUnifiedTools()

	// Give worker time to initialize
	time.Sleep(10 * time.Millisecond)

	// Test 1: Register and retrieve tools
	numTools := 5
	for i := 0; i < numTools; i++ {
		tool := &MockBuiltinTool{
			name:        fmt.Sprintf("comprehensive_tool_%d", i),
			description: fmt.Sprintf("Comprehensive test tool %d", i),
		}
		registry.Register(tool)
	}

	// Give time for all registrations to complete
	time.Sleep(20 * time.Millisecond)

	// Test 2: Verify individual GetTool works
	for i := 0; i < numTools; i++ {
		toolName := fmt.Sprintf("comprehensive_tool_%d", i)
		tool, found := registry.GetTool(toolName)
		if !found {
			t.Errorf("Should find tool '%s'", toolName)
		}
		if tool == nil {
			t.Errorf("Tool '%s' should not be nil", toolName)
		} else if tool.Name() != toolName {
			t.Errorf("Expected tool name '%s', got '%s'", toolName, tool.Name())
		}
	}

	// Test 3: Verify GetAllTools works
	allTools := registry.GetAllTools()
	if len(allTools) != numTools {
		t.Errorf("Expected %d tools, got %d", numTools, len(allTools))
	}

	// Test 4: Verify unified tool operations
	for i := 0; i < numTools; i++ {
		toolName := fmt.Sprintf("comprehensive_tool_%d", i)
		unifiedTool, found := registry.GetUnifiedTool(toolName)
		if !found {
			t.Errorf("Should find unified tool '%s'", toolName)
		}
		if unifiedTool == nil {
			t.Errorf("Unified tool '%s' should not be nil", toolName)
		} else {
			if unifiedTool.Name != toolName {
				t.Errorf("Expected unified tool name '%s', got '%s'", toolName, unifiedTool.Name)
			}
			if unifiedTool.Source != "builtin" {
				t.Errorf("Expected source 'builtin', got '%s'", unifiedTool.Source)
			}
		}
	}

	// Test 5: Verify GetAllUnifiedTools works
	allUnifiedTools := registry.GetAllUnifiedTools()
	if len(allUnifiedTools) != numTools {
		t.Errorf("Expected %d unified tools, got %d", numTools, len(allUnifiedTools))
	}

	// Test 6: Concurrent access
	const numGoroutines = 10
	const numOperationsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperationsPerGoroutine; j++ {
				toolIndex := j % numTools
				toolName := fmt.Sprintf("comprehensive_tool_%d", toolIndex)

				// Mix of operations
				switch j % 4 {
				case 0:
					registry.GetTool(toolName)
				case 1:
					registry.GetAllTools()
				case 2:
					registry.GetUnifiedTool(toolName)
				case 3:
					registry.GetAllUnifiedTools()
				}
			}
		}(i)
	}

	wg.Wait()

	// Final verification after concurrent access
	finalTools := registry.GetAllTools()
	if len(finalTools) != numTools {
		t.Errorf("After concurrent access: Expected %d tools, got %d", numTools, len(finalTools))
	}
}

// Test pure mutex operations (baseline)
func TestToolRegistry_AllMutexOperations_Baseline(t *testing.T) {
	registry := NewToolRegistry()
	// No channel operations enabled - pure mutex

	// Register tools
	numTools := 5
	for i := 0; i < numTools; i++ {
		tool := &MockBuiltinTool{
			name:        fmt.Sprintf("mutex_tool_%d", i),
			description: fmt.Sprintf("Mutex test tool %d", i),
		}
		registry.Register(tool)
	}

	// Test all operations work with pure mutex
	for i := 0; i < numTools; i++ {
		toolName := fmt.Sprintf("mutex_tool_%d", i)

		// GetTool
		tool, found := registry.GetTool(toolName)
		if !found || tool == nil {
			t.Errorf("Mutex GetTool failed for '%s'", toolName)
		}

		// GetUnifiedTool
		unifiedTool, found := registry.GetUnifiedTool(toolName)
		if !found || unifiedTool == nil {
			t.Errorf("Mutex GetUnifiedTool failed for '%s'", toolName)
		}
	}

	// GetAllTools
	allTools := registry.GetAllTools()
	if len(allTools) != numTools {
		t.Errorf("Mutex GetAllTools: Expected %d tools, got %d", numTools, len(allTools))
	}

	// GetAllUnifiedTools
	allUnifiedTools := registry.GetAllUnifiedTools()
	if len(allUnifiedTools) != numTools {
		t.Errorf("Mutex GetAllUnifiedTools: Expected %d tools, got %d", numTools, len(allUnifiedTools))
	}
}

// Test that demonstrates the channel operations work when consistently used
func TestToolRegistry_ChannelConsistency(t *testing.T) {
	tests := []struct {
		name       string
		operations []string
	}{
		{
			name:       "GetTool_and_Register",
			operations: []string{"GetTool", "Register"},
		},
		{
			name:       "All_Builtin_Operations",
			operations: []string{"GetTool", "Register", "GetAllTools"},
		},
		{
			name:       "All_Unified_Operations",
			operations: []string{"GetUnifiedTool", "GetAllUnifiedTools"},
		},
		{
			name:       "All_Operations",
			operations: []string{"GetTool", "Register", "GetAllTools", "SetMCPManager", "GetUnifiedTool", "GetAllUnifiedTools"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()
			defer registry.DisableChannelOperations()

			// Enable only the operations being tested
			for _, op := range tt.operations {
				switch op {
				case "GetTool":
					registry.EnableChannelForGetTool()
				case "Register":
					registry.EnableChannelForRegister()
				case "GetAllTools":
					registry.EnableChannelForGetAllTools()
				case "GetUnifiedTool":
					registry.EnableChannelForGetUnifiedTool()
				case "GetAllUnifiedTools":
					registry.EnableChannelForGetAllUnifiedTools()
				case "SetMCPManager":
					registry.EnableChannelForSetMCPManager()
				}
			}

			time.Sleep(10 * time.Millisecond)

			// Register test tools
			numTools := 3
			for i := 0; i < numTools; i++ {
				tool := &MockBuiltinTool{
					name:        fmt.Sprintf("consistency_tool_%d", i),
					description: fmt.Sprintf("Consistency test tool %d", i),
				}
				registry.Register(tool)
			}

			time.Sleep(10 * time.Millisecond)

			// Test the enabled operations
			for _, op := range tt.operations {
				switch op {
				case "GetTool":
					tool, found := registry.GetTool("consistency_tool_0")
					if !found {
						t.Errorf("GetTool failed in test '%s'", tt.name)
					}
					if tool == nil || tool.Name() != "consistency_tool_0" {
						t.Errorf("GetTool returned invalid tool in test '%s'", tt.name)
					}

				case "GetAllTools":
					allTools := registry.GetAllTools()
					if len(allTools) != numTools {
						t.Errorf("GetAllTools failed in test '%s': expected %d, got %d", tt.name, numTools, len(allTools))
					}

				case "GetUnifiedTool":
					unifiedTool, found := registry.GetUnifiedTool("consistency_tool_0")
					if !found {
						t.Errorf("GetUnifiedTool failed in test '%s'", tt.name)
					}
					if unifiedTool == nil || unifiedTool.Name != "consistency_tool_0" {
						t.Errorf("GetUnifiedTool returned invalid tool in test '%s'", tt.name)
					}

				case "GetAllUnifiedTools":
					allUnified := registry.GetAllUnifiedTools()
					if len(allUnified) != numTools {
						t.Errorf("GetAllUnifiedTools failed in test '%s': expected %d, got %d", tt.name, numTools, len(allUnified))
					}
				}
			}
		})
	}
}
