package tooling

import (
	"testing"
	"time"

	"github.com/ollama/ollama/api"
)

// testMetricsTool is a simple test tool for metrics testing
type testMetricsTool struct{}

func (t *testMetricsTool) Name() string        { return "metrics_test_tool" }
func (t *testMetricsTool) Description() string { return "Tool for testing metrics" }
func (t *testMetricsTool) Execute(args map[string]any) (any, error) {
	return "test result", nil
}
func (t *testMetricsTool) GetAPITool() *api.Tool {
	return &api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters: api.ToolFunctionParameters{
				Type:       "object",
				Properties: make(map[string]api.ToolProperty),
			},
		},
	}
}

// TestPerformanceMetrics tests that performance monitoring works correctly
func TestPerformanceMetrics(t *testing.T) {
	// Create test tool - using a simple mock since fields are unexported
	testTool := &testMetricsTool{}

	// Test with mixed channel/mutex operations
	registry := NewToolRegistry()
	defer registry.DisableChannelOperations()

	// Enable only GetTool for channels (mixed usage)
	registry.EnableChannelForGetTool()
	registry.Register(testTool) // Uses mutex

	// Initial metrics should be minimal
	channelOps, mutexOps, syncOps, channelLatency, mutexLatency := registry.GetPerformanceMetrics()
	t.Logf("Initial metrics: channel=%d, mutex=%d, sync=%d, channel_latency=%v, mutex_latency=%v",
		channelOps, mutexOps, syncOps, channelLatency, mutexLatency)

	// Perform some operations
	for i := 0; i < 10; i++ {
		// This should use channels
		tool, found := registry.GetTool("metrics_test_tool")
		if !found || tool == nil {
			t.Errorf("Should find tool via channel operation %d", i)
		}
	}

	// Get updated metrics
	channelOps, mutexOps, syncOps, channelLatency, mutexLatency = registry.GetPerformanceMetrics()
	t.Logf("After channel ops: channel=%d, mutex=%d, sync=%d, channel_latency=%v, mutex_latency=%v",
		channelOps, mutexOps, syncOps, channelLatency, mutexLatency)

	// Should have recorded channel operations
	if channelOps == 0 {
		t.Error("Should have recorded channel operations")
	}

	if channelLatency == 0 {
		t.Error("Should have recorded channel latency")
	}

	// Now disable channels and do mutex operations
	registry.DisableChannelOperations()
	registry = NewToolRegistry() // Fresh registry for mutex only
	defer registry.DisableChannelOperations()

	registry.Register(testTool) // Uses mutex

	for i := 0; i < 5; i++ {
		// This should use mutex
		tool, found := registry.GetTool("metrics_test_tool")
		if !found || tool == nil {
			t.Errorf("Should find tool via mutex operation %d", i)
		}
	}

	// Get final metrics
	channelOps, mutexOps, syncOps, channelLatency, mutexLatency = registry.GetPerformanceMetrics()
	t.Logf("After mutex ops: channel=%d, mutex=%d, sync=%d, channel_latency=%v, mutex_latency=%v",
		channelOps, mutexOps, syncOps, channelLatency, mutexLatency)

	// Should have recorded mutex operations
	if mutexOps == 0 {
		t.Error("Should have recorded mutex operations")
	}

	if mutexLatency == 0 {
		t.Error("Should have recorded mutex latency")
	}
}

// TestPerformanceMetricsExport tests the metrics export functionality
func TestPerformanceMetricsExport(t *testing.T) {
	registry := NewToolRegistry()
	defer registry.DisableChannelOperations()

	// Enable mixed operations
	registry.EnableChannelForGetTool()
	registry.EnableChannelForRegister()

	testTool := &testMetricsTool{}

	// Perform operations to generate metrics
	registry.Register(testTool)

	for i := 0; i < 3; i++ {
		registry.GetTool("export_test_tool")
	}

	// Wait a bit to ensure operations complete
	time.Sleep(10 * time.Millisecond)

	// Get and validate metrics
	channelOps, mutexOps, syncOps, channelLatency, mutexLatency := registry.GetPerformanceMetrics()

	t.Logf("Final metrics: channel_ops=%d, mutex_ops=%d, sync_ops=%d", channelOps, mutexOps, syncOps)
	t.Logf("Latencies: channel=%v, mutex=%v", channelLatency, mutexLatency)

	// Validate metrics are reasonable
	if channelOps < 0 || mutexOps < 0 || syncOps < 0 {
		t.Error("Operation counts should not be negative")
	}

	if channelLatency < 0 || mutexLatency < 0 {
		t.Error("Latencies should not be negative")
	}

	// If we had channel operations, latency should be positive
	if channelOps > 0 && channelLatency == 0 {
		t.Error("Channel latency should be positive when operations occurred")
	}
}
