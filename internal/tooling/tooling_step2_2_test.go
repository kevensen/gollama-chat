package tooling

import (
	"testing"
)

// Test Step 2.2: Channel-based Register operation
func TestToolRegistry_Step2_2_RegisterViaChannels(t *testing.T) {
	registry := NewToolRegistry()
	defer registry.DisableChannelOperations()

	// Create a mock tool for testing
	mockTool := &MockBuiltinTool{
		name:        "step2-2-test",
		description: "Step 2.2 test tool",
	}

	// Test 1: Register via mutex (channels disabled)
	registry.Register(mockTool)

	// Should be able to retrieve via mutex GetTool
	tool, found := registry.GetTool("step2-2-test")
	if !found {
		t.Fatal("Should find tool registered via mutex")
	}
	if tool.Name() != "step2-2-test" {
		t.Fatalf("Expected 'step2-2-test', got '%s'", tool.Name())
	}

	// Test 2: Enable channel-based Register
	registry.EnableChannelForRegister()

	// Register new tool via channels
	mockTool2 := &MockBuiltinTool{
		name:        "step2-2-channel-test",
		description: "Step 2.2 channel test tool",
	}
	registry.Register(mockTool2)

	// Test 3: Enable channel-based GetTool to retrieve channel-registered tool
	registry.EnableChannelForGetTool()
	tool2, found2 := registry.GetTool("step2-2-channel-test")
	if !found2 {
		t.Fatal("Should find tool registered via channel")
	}
	if tool2.Name() != "step2-2-channel-test" {
		t.Fatalf("Expected 'step2-2-channel-test', got '%s'", tool2.Name())
	}

	// Test 4: Both operations via channels should be consistent
	mockTool3 := &MockBuiltinTool{
		name:        "consistency-test",
		description: "Consistency test tool",
	}
	registry.Register(mockTool3)

	tool3, found3 := registry.GetTool("consistency-test")
	if !found3 {
		t.Fatal("Should find tool for consistency test")
	}
	if tool3.Name() != "consistency-test" {
		t.Fatalf("Expected 'consistency-test', got '%s'", tool3.Name())
	}
}

// Test both GetTool and Register operations work together via channels
func TestToolRegistry_Step2_2_Integration(t *testing.T) {
	registry := NewToolRegistry()
	defer registry.DisableChannelOperations()

	// Enable both channel operations
	registry.EnableChannelForGetTool()
	registry.EnableChannelForRegister()

	// Register multiple tools via channels
	tools := []*MockBuiltinTool{
		{name: "integration-1", description: "Integration test 1"},
		{name: "integration-2", description: "Integration test 2"},
		{name: "integration-3", description: "Integration test 3"},
	}

	for _, tool := range tools {
		registry.Register(tool)
	}

	// Retrieve all tools via channels
	for _, expectedTool := range tools {
		retrievedTool, found := registry.GetTool(expectedTool.name)
		if !found {
			t.Fatalf("Should find tool '%s'", expectedTool.name)
		}
		if retrievedTool.Name() != expectedTool.name {
			t.Fatalf("Expected '%s', got '%s'", expectedTool.name, retrievedTool.Name())
		}
	}

	// Test non-existent tool
	_, found := registry.GetTool("non-existent")
	if found {
		t.Fatal("Should not find non-existent tool")
	}
}
