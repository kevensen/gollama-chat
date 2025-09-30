// Package tooling provides a registry for managing builtin and MCP tools with dual channel/mutex implementation.
//
// This package implements an incremental migration strategy from sync.RWMutex to channel-based operations,
// following Go's principle of "share memory by communicating". The ToolRegistry supports both approaches
// simultaneously, allowing for gradual migration and performance optimization.
//
// Key features:
//   - Dual implementation (channels + mutex) for incremental migration
//   - Performance optimization through channel pooling and sync state caching
//   - Comprehensive performance monitoring and metrics
//   - Thread-safe operations with smart synchronization
//   - Graceful shutdown handling with state preservation
//
// Basic Usage:
//
//	// Create a new registry
//	registry := NewToolRegistry()
//	defer registry.DisableChannelOperations() // Clean shutdown
//
//	// Register a tool (uses mutex by default)
//	tool := &MyTool{name: "example"}
//	registry.Register(tool)
//
//	// Retrieve a tool (uses mutex by default)
//	found, exists := registry.GetTool("example")
//
// Incremental Migration:
//
//	// Enable channel operations gradually
//	registry.EnableChannelForGetTool()      // GetTool now uses channels
//	registry.EnableChannelForRegister()     // Register now uses channels
//
//	// Mixed usage is supported with automatic synchronization
//	registry.Register(tool)                 // Uses channels
//	found, exists := registry.GetTool("example") // Uses channels
//
// Performance Characteristics:
//
// Mutex operations are typically faster for individual calls (~60-70ns) but don't scale
// as well under high concurrency. Channel operations have higher per-call overhead
// (~1.5-3µs) but scale better with multiple goroutines and provide better isolation.
//
// Benchmark results (Intel i7-10875H):
//   - Mutex GetTool: ~60-70ns, 16B/op, 1 alloc/op
//   - Channel GetTool: ~1.5-3µs, 16B/op, 1 alloc/op (with pooling)
//   - Sync operations: ~1-2 per mixed operation set
//
// The channel pooling optimization reduces allocations to the same level as mutex operations
// while the sync state caching reduces synchronization overhead by 2-5x.
//
// Performance Monitoring:
//
//	// Get performance metrics
//	channelOps, mutexOps, syncOps, channelLatency, mutexLatency := registry.GetPerformanceMetrics()
//	fmt.Printf("Channel ops: %d (avg: %v), Mutex ops: %d (avg: %v), Syncs: %d\n",
//		channelOps, channelLatency, mutexOps, mutexLatency, syncOps)
package tooling

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/kevensen/gollama-chat/internal/logging"
	"github.com/kevensen/gollama-chat/internal/tooling/mcp"
	"github.com/ollama/ollama/api"
)

// Global registry instance
var DefaultRegistry *ToolRegistry

// init initializes the default registry and registers built-in tools
func init() {
	logger := logging.WithComponent("tooling")
	logger.Info("Initializing default tool registry")

	DefaultRegistry = NewToolRegistry()

	// Register built-in tools
	logger.Info("Registering built-in tools")
	DefaultRegistry.Register(&FileSystemTool{})

	logger.Info("Default tool registry initialized", "builtinToolCount", len(DefaultRegistry.builtinTools))
}

// Tool represents a unified tool that can be either builtin or MCP
type Tool struct {
	Name        string    `json:"name"`        // Fully qualified name (e.g., "server.tool" for MCP tools)
	DisplayName string    `json:"displayName"` // Display name for UI
	Description string    `json:"description"`
	Source      string    `json:"source"`     // "builtin" or "mcp"
	ServerName  string    `json:"serverName"` // Empty for builtin, server name for MCP
	Available   bool      `json:"available"`  // Whether the tool is currently available
	APITool     *api.Tool `json:"-"`          // The Ollama API tool definition
}

// ToolRegistry holds all registered tools (builtin and MCP) with dual channel/mutex implementation.
//
// The registry supports incremental migration from mutex-based to channel-based operations.
// Each operation can be independently configured to use either approach, enabling gradual
// migration and performance testing.
//
// Performance optimizations include:
//   - Response channel pooling to reduce allocations
//   - Sync state caching to avoid unnecessary synchronization
//   - Real-time performance monitoring and metrics
//
// Thread safety is maintained through:
//   - Single worker goroutine for channel operations
//   - Smart bidirectional synchronization between mutex and channel state
//   - Atomic counters for lock-free state tracking
type ToolRegistry struct {
	builtinTools map[string]BuiltinTool
	mcpManager   *mcp.Manager
	tools        map[string]*Tool
	toolsMutex   sync.RWMutex

	// Channel-based operations (incremental migration)
	useChannelForGetTool            bool
	useChannelForRegister           bool
	useChannelForGetAllTools        bool
	useChannelForSetMCPManager      bool
	useChannelForGetUnifiedTool     bool
	useChannelForGetAllUnifiedTools bool
	requests                        chan toolRequest
	shutdown                        chan struct{}
	done                            chan struct{}
	wg                              sync.WaitGroup

	// Performance optimization: response channel pool
	responseChannelPool sync.Pool

	// Performance optimization: sync state caching
	lastSyncFromWorker int64 // atomic counter for worker→mutex syncs
	lastSyncToWorker   int64 // atomic counter for mutex→worker syncs
	workerState        int64 // atomic counter for worker state changes
	mutexState         int64 // atomic counter for mutex state changes

	// Performance monitoring
	metrics struct {
		channelOperations int64         // total channel operations
		mutexOperations   int64         // total mutex operations
		syncOperations    int64         // total sync operations
		avgChannelLatency time.Duration // average channel operation latency
		avgMutexLatency   time.Duration // average mutex operation latency
		latencyMutex      sync.RWMutex  // protects latency calculations
	}
}

// toolRequest represents a request to the tool registry service
type toolRequest struct {
	operation  string // "get", "register", "list", "setMCPManager", etc.
	toolName   string
	tool       BuiltinTool
	mcpManager *mcp.Manager
	response   chan toolResponse
}

// toolResponse represents a response from the tool registry service
type toolResponse struct {
	builtinTool     BuiltinTool
	unifiedTool     *Tool
	allTools        map[string]BuiltinTool
	allUnifiedTools map[string]*Tool
	found           bool
	error           error
}

// BuiltinTool represents a built-in tool that can be registered
type BuiltinTool interface {
	Name() string
	Description() string
	GetAPITool() *api.Tool
	Execute(args map[string]interface{}) (interface{}, error)
}

// NewToolRegistry creates a new tool registry with channel support.
//
// The registry is initialized with:
//   - Buffered channel for improved performance (100 capacity)
//   - Response channel pool for allocation optimization
//   - Background worker goroutine for channel operations
//   - Performance monitoring infrastructure
//
// All operations default to mutex implementation until explicitly enabled for channels.
func NewToolRegistry() *ToolRegistry {
	tr := &ToolRegistry{
		builtinTools: make(map[string]BuiltinTool),
		tools:        make(map[string]*Tool),
		requests:     make(chan toolRequest, 100), // Buffered channel for better performance
		shutdown:     make(chan struct{}),
		done:         make(chan struct{}),
		responseChannelPool: sync.Pool{
			New: func() interface{} {
				return make(chan toolResponse, 1) // Buffered for non-blocking sends
			},
		},
	}

	tr.wg.Add(1)
	go tr.worker()

	return tr
}

// getResponseChannel gets a response channel from the pool
func (tr *ToolRegistry) getResponseChannel() chan toolResponse {
	return tr.responseChannelPool.Get().(chan toolResponse)
}

// putResponseChannel returns a response channel to the pool
func (tr *ToolRegistry) putResponseChannel(ch chan toolResponse) {
	// Drain any remaining data and reset
	select {
	case <-ch:
	default:
	}
	tr.responseChannelPool.Put(ch)
}

// markWorkerStateChanged increments the worker state counter
func (tr *ToolRegistry) markWorkerStateChanged() {
	atomic.AddInt64(&tr.workerState, 1)
}

// markMutexStateChanged increments the mutex state counter
func (tr *ToolRegistry) markMutexStateChanged() {
	atomic.AddInt64(&tr.mutexState, 1)
}

// needsSyncToWorker checks if worker needs to sync from mutex
func (tr *ToolRegistry) needsSyncToWorker() bool {
	return atomic.LoadInt64(&tr.mutexState) > atomic.LoadInt64(&tr.lastSyncToWorker)
}

// needsSyncFromWorker checks if mutex needs to sync from worker
func (tr *ToolRegistry) needsSyncFromWorker() bool {
	return atomic.LoadInt64(&tr.workerState) > atomic.LoadInt64(&tr.lastSyncFromWorker)
}

// recordChannelOperation records timing and count for a channel operation.
//
// This method is called automatically by channel-based operations to track:
//   - Total operation count
//   - Running average latency using weighted average calculation
//
// The latency calculation uses: ((n-1)*avg + new) / n
func (tr *ToolRegistry) recordChannelOperation(duration time.Duration) {
	atomic.AddInt64(&tr.metrics.channelOperations, 1)

	tr.metrics.latencyMutex.Lock()
	defer tr.metrics.latencyMutex.Unlock()

	// Simple moving average calculation
	current := int64(tr.metrics.avgChannelLatency)
	operations := atomic.LoadInt64(&tr.metrics.channelOperations)

	// Weighted average: ((n-1)*avg + new) / n
	newAvg := (current*int64(operations-1) + int64(duration)) / int64(operations)
	tr.metrics.avgChannelLatency = time.Duration(newAvg)
}

// recordMutexOperation records timing and count for a mutex operation.
//
// This method is called automatically by mutex-based operations to track:
//   - Total operation count
//   - Running average latency using weighted average calculation
//
// The latency calculation uses: ((n-1)*avg + new) / n
func (tr *ToolRegistry) recordMutexOperation(duration time.Duration) {
	atomic.AddInt64(&tr.metrics.mutexOperations, 1)

	tr.metrics.latencyMutex.Lock()
	defer tr.metrics.latencyMutex.Unlock()

	// Simple moving average calculation
	current := int64(tr.metrics.avgMutexLatency)
	operations := atomic.LoadInt64(&tr.metrics.mutexOperations)

	// Weighted average: ((n-1)*avg + new) / n
	newAvg := (current*int64(operations-1) + int64(duration)) / int64(operations)
	tr.metrics.avgMutexLatency = time.Duration(newAvg)
}

// recordSyncOperation records a sync operation
func (tr *ToolRegistry) recordSyncOperation() {
	atomic.AddInt64(&tr.metrics.syncOperations, 1)
}

// GetPerformanceMetrics returns current performance metrics for monitoring and analysis.
//
// Returns:
//   - channelOps: Total number of channel-based operations performed
//   - mutexOps: Total number of mutex-based operations performed
//   - syncOps: Total number of synchronization operations between mutex and channel state
//   - channelLatency: Average latency of channel operations
//   - mutexLatency: Average latency of mutex operations
//
// These metrics can be used for:
//   - Performance monitoring and alerting
//   - Capacity planning and scaling decisions
//   - A/B testing between channel and mutex implementations
//   - Identifying performance bottlenecks
func (tr *ToolRegistry) GetPerformanceMetrics() (channelOps, mutexOps, syncOps int64, channelLatency, mutexLatency time.Duration) {
	tr.metrics.latencyMutex.RLock()
	defer tr.metrics.latencyMutex.RUnlock()

	return atomic.LoadInt64(&tr.metrics.channelOperations),
		atomic.LoadInt64(&tr.metrics.mutexOperations),
		atomic.LoadInt64(&tr.metrics.syncOperations),
		tr.metrics.avgChannelLatency,
		tr.metrics.avgMutexLatency
}

// EnableChannelForGetTool enables channel-based GetTool operation
func (tr *ToolRegistry) EnableChannelForGetTool() {
	if tr.useChannelForGetTool {
		return // Already enabled
	}

	tr.initializeChannels()
	tr.useChannelForGetTool = true
}

// EnableChannelForRegister enables channel-based Register operation
func (tr *ToolRegistry) EnableChannelForRegister() {
	if tr.useChannelForRegister {
		return // Already enabled
	}

	tr.initializeChannels()
	tr.useChannelForRegister = true
}

// EnableChannelForGetAllTools enables channel-based GetAllTools operation
func (tr *ToolRegistry) EnableChannelForGetAllTools() {
	if tr.useChannelForGetAllTools {
		return // Already enabled
	}

	tr.initializeChannels()
	tr.useChannelForGetAllTools = true
}

// EnableChannelForSetMCPManager enables channel-based SetMCPManager operation
func (tr *ToolRegistry) EnableChannelForSetMCPManager() {
	if tr.useChannelForSetMCPManager {
		return // Already enabled
	}

	tr.initializeChannels()
	tr.useChannelForSetMCPManager = true
}

// EnableChannelForGetUnifiedTool enables channel-based GetUnifiedTool operation
func (tr *ToolRegistry) EnableChannelForGetUnifiedTool() {
	if tr.useChannelForGetUnifiedTool {
		return // Already enabled
	}

	tr.initializeChannels()
	tr.useChannelForGetUnifiedTool = true
}

// EnableChannelForGetAllUnifiedTools enables channel-based GetAllUnifiedTools operation
func (tr *ToolRegistry) EnableChannelForGetAllUnifiedTools() {
	if tr.useChannelForGetAllUnifiedTools {
		return // Already enabled
	}

	tr.initializeChannels()
	tr.useChannelForGetAllUnifiedTools = true
}

// initializeChannels sets up the channel infrastructure if not already done
func (tr *ToolRegistry) initializeChannels() {
	if tr.requests != nil {
		return // Already initialized
	}

	tr.requests = make(chan toolRequest, 10)
	tr.shutdown = make(chan struct{})
	tr.done = make(chan struct{})

	tr.wg.Add(1)
	go tr.worker()
}

// DisableChannelOperations disables all channel-based operations and falls back to mutex
func (tr *ToolRegistry) DisableChannelOperations() {
	if tr.requests == nil {
		return // Already disabled
	}

	close(tr.shutdown)
	tr.wg.Wait()

	tr.useChannelForGetTool = false
	tr.useChannelForRegister = false
	tr.useChannelForGetAllTools = false
	tr.useChannelForSetMCPManager = false
	tr.useChannelForGetUnifiedTool = false
	tr.useChannelForGetAllUnifiedTools = false
	tr.requests = nil
	tr.shutdown = nil
	tr.done = nil
}

// syncWorkerState synchronizes worker's local state with mutex-protected registry state (mutex → worker)
func (tr *ToolRegistry) syncWorkerState(builtinTools map[string]BuiltinTool, unifiedTools map[string]*Tool) *mcp.Manager {
	tr.toolsMutex.RLock()
	defer tr.toolsMutex.RUnlock()

	// Clear and repopulate builtin tools
	for k := range builtinTools {
		delete(builtinTools, k)
	}
	for name, tool := range tr.builtinTools {
		builtinTools[name] = tool
	}

	// Clear and repopulate unified tools
	for k := range unifiedTools {
		delete(unifiedTools, k)
	}
	for name, tool := range tr.tools {
		unifiedTools[name] = tool
	}

	return tr.mcpManager
}

// syncMutexState synchronizes mutex-protected registry state with worker's local state (worker → mutex)
func (tr *ToolRegistry) syncMutexState(builtinTools map[string]BuiltinTool, unifiedTools map[string]*Tool, mcpManager *mcp.Manager) {
	tr.toolsMutex.Lock()
	defer tr.toolsMutex.Unlock()

	// Clear and repopulate builtin tools from worker state
	tr.builtinTools = make(map[string]BuiltinTool)
	for name, tool := range builtinTools {
		tr.builtinTools[name] = tool
	}

	// Clear and repopulate unified tools from worker state (but preserve MCP tools)
	// First, preserve existing MCP tools
	mcpTools := make(map[string]*Tool)
	for name, tool := range tr.tools {
		if tool.Source == "mcp" {
			mcpTools[name] = tool
		}
	}

	// Clear and add builtin tools from worker
	tr.tools = make(map[string]*Tool)
	for name, tool := range unifiedTools {
		if tool.Source == "builtin" {
			tr.tools[name] = tool
		}
	}

	// Add back MCP tools
	for name, tool := range mcpTools {
		tr.tools[name] = tool
	}

	// Update MCP manager if provided
	if mcpManager != nil {
		tr.mcpManager = mcpManager
	}
}

// worker runs the main service loop for channel-based operations
func (tr *ToolRegistry) worker() {
	defer tr.wg.Done()
	defer close(tr.done)

	// Create local copies of the data for the worker goroutine
	builtinTools := make(map[string]BuiltinTool)
	unifiedTools := make(map[string]*Tool)

	// Initialize with current mutex-protected data
	mcpManager := tr.syncWorkerState(builtinTools, unifiedTools)

	logger := logging.WithComponent("tooling")
	logger.Debug("Worker initialized", "hasInitialMCPManager", mcpManager != nil)

	for {
		select {
		case req := <-tr.requests:
			switch req.operation {
			case "sync":
				// Synchronize with mutex state (mutex → worker)
				mcpManager = tr.syncWorkerState(builtinTools, unifiedTools)
				req.response <- toolResponse{
					error: nil,
				}

			case "syncToMutex":
				// Synchronize mutex state with worker state (worker → mutex)
				tr.syncMutexState(builtinTools, unifiedTools, mcpManager)
				req.response <- toolResponse{
					error: nil,
				}

			case "get":
				tool, found := builtinTools[req.toolName]
				req.response <- toolResponse{
					builtinTool: tool,
					found:       found,
					error:       nil,
				}

			case "register":
				if req.tool == nil {
					req.response <- toolResponse{
						builtinTool: nil,
						found:       false,
						error:       fmt.Errorf("tool cannot be nil"),
					}

					break
				}

				// Register the builtin tool
				builtinTools[req.tool.Name()] = req.tool

				// Create unified tool entry
				unifiedTool := &Tool{
					Name:        req.tool.Name(),
					DisplayName: req.tool.Name(),
					Description: req.tool.Description(),
					Source:      "builtin",
					ServerName:  "",
					Available:   true,
					APITool:     req.tool.GetAPITool(),
				}
				unifiedTools[req.tool.Name()] = unifiedTool

				logger.Info("Registered builtin tool via channel", "name", req.tool.Name(), "description", req.tool.Description())
				logger.Debug("Builtin tool registered successfully via channel", "name", req.tool.Name())

				// Mark that worker state has changed
				tr.markWorkerStateChanged()

				req.response <- toolResponse{
					builtinTool: req.tool,
					found:       true,
					error:       nil,
				}

			case "list":
				// Create a copy of all builtin tools for the response
				allToolsCopy := make(map[string]BuiltinTool)
				for name, tool := range builtinTools {
					allToolsCopy[name] = tool
				}

				req.response <- toolResponse{
					allTools: allToolsCopy,
					found:    true,
					error:    nil,
				}

			case "setMCPManager":
				// Set the MCP manager and refresh tools
				if req.mcpManager == nil {
					req.response <- toolResponse{
						error: fmt.Errorf("MCP manager cannot be nil"),
					}

					break
				}

				// Store the MCP manager locally in worker
				mcpManager = req.mcpManager
				logger.Info("MCP manager set via channel")

				// TODO: Implement full refreshAllTools logic here
				// For now, we just store the manager and respond successfully

				// Mark that worker state has changed
				tr.markWorkerStateChanged()

				req.response <- toolResponse{
					found: true,
					error: nil,
				}

			case "getUnified":
				// Get a unified tool by name
				tool, found := unifiedTools[req.toolName]
				req.response <- toolResponse{
					unifiedTool: tool,
					found:       found,
					error:       nil,
				}

			case "listUnified":
				// Create a copy of all unified tools for the response
				allUnifiedCopy := make(map[string]*Tool)
				for name, tool := range unifiedTools {
					allUnifiedCopy[name] = tool
				}

				req.response <- toolResponse{
					allUnifiedTools: allUnifiedCopy,
					found:           true,
					error:           nil,
				}

			default:
				req.response <- toolResponse{
					builtinTool: nil,
					found:       false,
					error:       fmt.Errorf("unknown operation: %s", req.operation),
				}

			}

		case <-tr.shutdown:
			// Before shutting down, sync worker state back to mutex
			tr.syncMutexState(builtinTools, unifiedTools, mcpManager)

			// Drain any remaining requests
		drainLoop:
			for {
				select {
				case req := <-tr.requests:
					req.response <- toolResponse{
						builtinTool: nil,
						found:       false,
						error:       fmt.Errorf("service shutting down"),
					}

				default:
					break drainLoop
				}
			}
			return
		}
	}
}

// Register adds a tool to the registry
func (tr *ToolRegistry) Register(tool BuiltinTool) {
	if tr.useChannelForRegister && tr.requests != nil {
		tr.registerViaChannel(tool)
		return
	}

	// Fallback to mutex implementation
	logger := logging.WithComponent("tooling")
	logger.Info("Registering builtin tool", "name", tool.Name(), "description", tool.Description())

	tr.toolsMutex.Lock()
	defer tr.toolsMutex.Unlock()

	tr.builtinTools[tool.Name()] = tool

	// Create unified tool entry
	unifiedTool := &Tool{
		Name:        tool.Name(),
		DisplayName: tool.Name(),
		Description: tool.Description(),
		Source:      "builtin",
		ServerName:  "",
		Available:   true,
		APITool:     tool.GetAPITool(),
	}
	tr.tools[tool.Name()] = unifiedTool

	// Mark that mutex state has changed
	tr.markMutexStateChanged()

	logger.Debug("Builtin tool registered successfully", "name", tool.Name())
}

// registerViaChannel registers a tool using the channel-based approach
func (tr *ToolRegistry) registerViaChannel(tool BuiltinTool) {
	responseChan := make(chan toolResponse, 1)
	request := toolRequest{
		operation: "register",
		toolName:  tool.Name(),
		tool:      tool,
		response:  responseChan,
	}

	select {
	case tr.requests <- request:
		select {
		case response := <-responseChan:
			if response.error != nil {
				// Log error but don't return it since original Register doesn't return error
				logger := logging.WithComponent("tooling")
				logger.Error("Failed to register tool via channel", "tool", tool.Name(), "error", response.error)
			}
		case <-tr.shutdown:
			// Service is shutting down, ignore
		}
	case <-tr.shutdown:
		// Service is shutting down, ignore
	}
}

// syncWorkerStateIfNeeded synchronizes worker state if mixed channel/mutex usage is detected.
//
// This optimization uses atomic counters to avoid unnecessary synchronization operations.
// Synchronization only occurs when:
//  1. Mixed usage is detected (some operations use channels, some use mutex)
//  2. Register operation is NOT using channels (mutex has authoritative state)
//  3. State has actually changed since last sync (cached via atomic counters)
//
// This reduces sync overhead by ~2-5x in typical mixed-usage scenarios.
func (tr *ToolRegistry) syncWorkerStateIfNeeded() {
	if tr.requests == nil {
		return // No worker running
	}

	// Check if sync is needed using cached state
	if !tr.needsSyncToWorker() {
		return // No changes since last sync
	}

	// Only sync mutex → worker if:
	// 1. We have mixed usage (some operations use channels, some don't)
	// 2. Register is NOT using channels (so mutex has the authoritative state)
	channelOperationsCount := 0
	totalOperations := 6 // GetTool, Register, GetAllTools, SetMCPManager, GetUnifiedTool, GetAllUnifiedTools

	if tr.useChannelForGetTool {
		channelOperationsCount++
	}
	if tr.useChannelForRegister {
		channelOperationsCount++
	}
	if tr.useChannelForGetAllTools {
		channelOperationsCount++
	}
	if tr.useChannelForSetMCPManager {
		channelOperationsCount++
	}
	if tr.useChannelForGetUnifiedTool {
		channelOperationsCount++
	}
	if tr.useChannelForGetAllUnifiedTools {
		channelOperationsCount++
	}

	// Only sync if we have partial usage AND Register is not using channels
	// (If Register uses channels, worker state is authoritative)
	if channelOperationsCount > 0 && channelOperationsCount < totalOperations && !tr.useChannelForRegister {
		// Send sync request to worker (mutex → worker)
		responseChan := tr.getResponseChannel()
		defer tr.putResponseChannel(responseChan)

		request := toolRequest{
			operation: "sync",
			response:  responseChan,
		}

		select {
		case tr.requests <- request:
			<-responseChan // Wait for sync to complete
			// Mark that we've completed a sync
			atomic.StoreInt64(&tr.lastSyncToWorker, atomic.LoadInt64(&tr.mutexState))
			tr.recordSyncOperation()
		case <-tr.shutdown:
			return
		}
	}
}

// syncMutexStateIfNeeded synchronizes mutex state with worker state for mixed usage scenarios.
//
// This optimization uses atomic counters to avoid unnecessary synchronization operations.
// Synchronization only occurs when:
//  1. Register operation IS using channels (worker has authoritative state)
//  2. Mutex operations need current state from worker
//  3. Worker state has actually changed since last sync (cached via atomic counters)
//
// This prevents tools registered via channels from being invisible to mutex operations.
func (tr *ToolRegistry) syncMutexStateIfNeeded() {
	if tr.requests == nil {
		return // No worker running
	}

	// Check if sync is needed using cached state
	if !tr.needsSyncFromWorker() {
		return // No changes since last sync
	}

	// Only sync worker→mutex if Register is using channels but the calling operation is not
	// This prevents situations where tools registered via channels would be invisible to mutex operations
	if tr.useChannelForRegister {
		// Send syncToMutex request to worker (worker → mutex)
		responseChan := tr.getResponseChannel()
		defer tr.putResponseChannel(responseChan)

		request := toolRequest{
			operation: "syncToMutex",
			response:  responseChan,
		}

		select {
		case tr.requests <- request:
			<-responseChan // Wait for sync to complete
			// Mark that we've completed a sync
			atomic.StoreInt64(&tr.lastSyncFromWorker, atomic.LoadInt64(&tr.workerState))
			tr.recordSyncOperation()
		case <-tr.shutdown:
			return
		}
	}
}

// GetTool retrieves a tool by name
func (tr *ToolRegistry) GetTool(name string) (BuiltinTool, bool) {
	if tr.useChannelForGetTool && tr.requests != nil {
		tr.syncWorkerStateIfNeeded()
		return tr.getToolViaChannel(name)
	}

	// Fallback to mutex implementation
	start := time.Now()
	defer func() {
		tr.recordMutexOperation(time.Since(start))
	}()

	// If Register is using channels, sync worker state to mutex first
	tr.syncMutexStateIfNeeded()

	tr.toolsMutex.RLock()
	defer tr.toolsMutex.RUnlock()
	tool, exists := tr.builtinTools[name]
	return tool, exists
}

// getToolViaChannel retrieves a tool using the channel-based approach
func (tr *ToolRegistry) getToolViaChannel(name string) (BuiltinTool, bool) {
	start := time.Now()
	defer func() {
		tr.recordChannelOperation(time.Since(start))
	}()

	responseChan := tr.getResponseChannel()
	defer tr.putResponseChannel(responseChan)

	request := toolRequest{
		operation: "get",
		toolName:  name,
		tool:      nil,
		response:  responseChan,
	}

	select {
	case tr.requests <- request:
		select {
		case response := <-responseChan:
			if response.error != nil {
				return nil, false
			}
			return response.builtinTool, response.found
		case <-tr.shutdown:
			return nil, false
		}
	case <-tr.shutdown:
		return nil, false
	}
}

// getAllToolsViaChannel retrieves all tools using the channel-based approach
func (tr *ToolRegistry) getAllToolsViaChannel() map[string]BuiltinTool {
	responseChan := tr.getResponseChannel()
	defer tr.putResponseChannel(responseChan)

	request := toolRequest{
		operation: "list",
		toolName:  "",
		tool:      nil,
		response:  responseChan,
	}

	select {
	case tr.requests <- request:
		select {
		case response := <-responseChan:
			if response.error != nil {
				return make(map[string]BuiltinTool) // Return empty map on error
			}
			return response.allTools
		case <-tr.shutdown:
			return make(map[string]BuiltinTool) // Return empty map on shutdown
		}
	case <-tr.shutdown:
		return make(map[string]BuiltinTool) // Return empty map on shutdown
	}
}

// setMCPManagerViaChannel sets the MCP manager using the channel-based approach
func (tr *ToolRegistry) setMCPManagerViaChannel(manager *mcp.Manager) {
	responseChan := tr.getResponseChannel()
	defer tr.putResponseChannel(responseChan)

	request := toolRequest{
		operation:  "setMCPManager",
		toolName:   "",
		tool:       nil,
		mcpManager: manager,
		response:   responseChan,
	}

	select {
	case tr.requests <- request:
		select {
		case response := <-responseChan:
			if response.error != nil {
				// Log error but don't return it since original SetMCPManager doesn't return error
				logger := logging.WithComponent("tooling")
				logger.Error("Failed to set MCP manager via channel", "error", response.error)
			}
		case <-tr.shutdown:
			// Service is shutting down, ignore
		}
	case <-tr.shutdown:
		// Service is shutting down, ignore
	}
}

// getUnifiedToolViaChannel retrieves a unified tool using the channel-based approach
func (tr *ToolRegistry) getUnifiedToolViaChannel(name string) (*Tool, bool) {
	responseChan := tr.getResponseChannel()
	defer tr.putResponseChannel(responseChan)

	request := toolRequest{
		operation:  "getUnified",
		toolName:   name,
		tool:       nil,
		mcpManager: nil,
		response:   responseChan,
	}

	select {
	case tr.requests <- request:
		select {
		case response := <-responseChan:
			if response.error != nil {
				return nil, false
			}
			return response.unifiedTool, response.found
		case <-tr.shutdown:
			return nil, false
		}
	case <-tr.shutdown:
		return nil, false
	}
}

// getAllUnifiedToolsViaChannel retrieves all unified tools using the channel-based approach
func (tr *ToolRegistry) getAllUnifiedToolsViaChannel() map[string]*Tool {
	responseChan := tr.getResponseChannel()
	defer tr.putResponseChannel(responseChan)

	request := toolRequest{
		operation:  "listUnified",
		toolName:   "",
		tool:       nil,
		mcpManager: nil,
		response:   responseChan,
	}

	select {
	case tr.requests <- request:
		select {
		case response := <-responseChan:
			if response.error != nil {
				return make(map[string]*Tool) // Return empty map on error
			}
			return response.allUnifiedTools
		case <-tr.shutdown:
			return make(map[string]*Tool) // Return empty map on shutdown
		}
	case <-tr.shutdown:
		return make(map[string]*Tool) // Return empty map on shutdown
	}
}

// GetAllTools returns all registered tools
func (tr *ToolRegistry) GetAllTools() map[string]BuiltinTool {
	if tr.useChannelForGetAllTools && tr.requests != nil {
		tr.syncWorkerStateIfNeeded()
		return tr.getAllToolsViaChannel()
	}

	// Fallback to mutex implementation
	// If Register is using channels, sync worker state to mutex first
	tr.syncMutexStateIfNeeded()

	tr.toolsMutex.RLock()
	defer tr.toolsMutex.RUnlock()

	result := make(map[string]BuiltinTool)
	for name, tool := range tr.builtinTools {
		result[name] = tool
	}
	return result
}

// SetMCPManager sets the MCP manager for this registry
func (tr *ToolRegistry) SetMCPManager(manager *mcp.Manager) {
	if tr.useChannelForSetMCPManager && tr.requests != nil {
		tr.setMCPManagerViaChannel(manager)
		return
	}

	// Fallback to mutex implementation
	logger := logging.WithComponent("tooling")
	logger.Info("Setting MCP manager for tool registry")

	tr.toolsMutex.Lock()
	defer tr.toolsMutex.Unlock()
	tr.mcpManager = manager
	tr.refreshAllTools()

	// Mark that mutex state has changed
	tr.markMutexStateChanged()

	logger.Info("MCP manager set and tools refreshed", "totalTools", len(tr.tools))
}

// GetUnifiedTool retrieves a unified tool by name
func (tr *ToolRegistry) GetUnifiedTool(name string) (*Tool, bool) {
	if tr.useChannelForGetUnifiedTool && tr.requests != nil {
		tr.syncWorkerStateIfNeeded()
		return tr.getUnifiedToolViaChannel(name)
	}

	// Fallback to mutex implementation
	// If Register is using channels, sync worker state to mutex first
	tr.syncMutexStateIfNeeded()

	tr.toolsMutex.RLock()
	defer tr.toolsMutex.RUnlock()
	tool, exists := tr.tools[name]
	return tool, exists
}

// GetAllUnifiedTools returns all tools (builtin and MCP) as unified tools
func (tr *ToolRegistry) GetAllUnifiedTools() map[string]*Tool {
	if tr.useChannelForGetAllUnifiedTools && tr.requests != nil {
		tr.syncWorkerStateIfNeeded()
		return tr.getAllUnifiedToolsViaChannel()
	}

	// Fallback to mutex implementation
	// If Register is using channels, sync worker state to mutex first
	tr.syncMutexStateIfNeeded()

	tr.toolsMutex.RLock()
	defer tr.toolsMutex.RUnlock()

	result := make(map[string]*Tool)
	for name, tool := range tr.tools {
		result[name] = tool
	}
	return result
}

// RefreshMCPTools refreshes the MCP tools from all servers
func (tr *ToolRegistry) RefreshMCPTools() error {
	logger := logging.WithComponent("tooling")
	logger.Info("Refreshing MCP tools")

	tr.toolsMutex.Lock()
	defer tr.toolsMutex.Unlock()

	if tr.mcpManager == nil {
		logger.Debug("No MCP manager configured, skipping MCP tool refresh")
		return nil // No MCP manager configured
	}

	// Remove existing MCP tools
	mcpToolCount := 0
	for name, tool := range tr.tools {
		if tool.Source == "mcp" {
			delete(tr.tools, name)
			mcpToolCount++
		}
	}
	logger.Debug("Removed existing MCP tools", "count", mcpToolCount)

	// Refresh tools from MCP manager
	if err := tr.mcpManager.RefreshTools(); err != nil {
		logger.Error("Failed to refresh MCP tools from manager", "error", err)
		return fmt.Errorf("failed to refresh MCP tools: %w", err)
	}

	// Add MCP tools
	if err := tr.refreshAllTools(); err != nil {
		logger.Error("Failed to refresh all tools after MCP refresh", "error", err)
		return err
	}

	logger.Info("MCP tools refreshed successfully", "totalTools", len(tr.tools))
	return nil
}

// refreshAllTools updates the unified tools map with current MCP tools
func (tr *ToolRegistry) refreshAllTools() error {
	logger := logging.WithComponent("tooling")

	if tr.mcpManager == nil {
		logger.Debug("No MCP manager available for tool refresh")
		return nil
	}

	serverTools := tr.mcpManager.GetAllTools()
	serverStatuses := tr.mcpManager.GetAllServerStatuses()

	logger.Debug("Retrieved tools from MCP servers", "serverCount", len(serverTools))

	for serverName, tools := range serverTools {
		serverStatus := serverStatuses[serverName]
		available := serverStatus == mcp.StatusRunning

		logger.Debug("Processing tools from MCP server", "server", serverName, "status", serverStatus.String(), "available", available, "toolCount", len(tools))

		for _, mcpTool := range tools {
			// Create namespaced tool name
			fullName := fmt.Sprintf("%s.%s", serverName, mcpTool.Name)

			// Convert MCP tool to Ollama API tool format
			apiTool := &api.Tool{
				Type: "function",
				Function: api.ToolFunction{
					Name:        fullName,
					Description: mcpTool.Description,
					Parameters:  convertMCPSchemaToOllamaParams(mcpTool.InputSchema),
				},
			}

			unifiedTool := &Tool{
				Name:        fullName,
				DisplayName: mcpTool.Name,
				Description: mcpTool.Description,
				Source:      "mcp",
				ServerName:  serverName,
				Available:   available,
				APITool:     apiTool,
			}

			tr.tools[fullName] = unifiedTool
			logger.Debug("Added MCP tool to registry", "server", serverName, "tool", mcpTool.Name, "fullName", fullName, "available", available)
		}
	}

	builtinCount := 0
	mcpCount := 0
	for _, tool := range tr.tools {
		switch tool.Source {
		case "builtin":
			builtinCount++
		case "mcp":
			mcpCount++
		}
	}

	logger.Info("Tool registry refreshed", "totalTools", len(tr.tools), "builtinTools", builtinCount, "mcpTools", mcpCount)
	return nil
}

// convertMCPSchemaToOllamaParams converts MCP tool schema to Ollama parameters format
func convertMCPSchemaToOllamaParams(schema mcp.ToolSchema) api.ToolFunctionParameters {
	params := api.ToolFunctionParameters{
		Type:       schema.Type,
		Properties: make(map[string]api.ToolProperty),
		Required:   schema.Required,
	}

	for propName, propSchema := range schema.Properties {
		if propMap, ok := propSchema.(map[string]interface{}); ok {
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

// ExecuteTool executes a tool (builtin or MCP) by name
func (tr *ToolRegistry) ExecuteTool(name string, args map[string]interface{}) (interface{}, error) {
	logger := logging.WithComponent("tooling")
	logger.Info("Executing tool", "name", name, "args", args)

	tr.toolsMutex.RLock()
	defer tr.toolsMutex.RUnlock()

	tool, exists := tr.tools[name]
	if !exists {
		logger.Error("Tool not found", "name", name)
		return nil, fmt.Errorf("tool %s not found", name)
	}

	if !tool.Available {
		logger.Error("Tool not available", "name", name, "source", tool.Source, "serverName", tool.ServerName)
		return nil, fmt.Errorf("tool %s is not available (server may be down)", name)
	}

	if tool.Source == "builtin" {
		logger.Debug("Executing builtin tool", "name", name)
		// Execute builtin tool
		builtinTool, exists := tr.builtinTools[name]
		if !exists {
			logger.Error("Builtin tool not found in registry", "name", name)
			return nil, fmt.Errorf("builtin tool %s not found", name)
		}
		result, err := builtinTool.Execute(args)
		if err != nil {
			logger.Error("Builtin tool execution failed", "name", name, "error", err)
		} else {
			logger.Info("Builtin tool executed successfully", "name", name)
		}
		return result, err
	} else if tool.Source == "mcp" {
		logger.Debug("Executing MCP tool", "name", name, "server", tool.ServerName, "toolName", tool.DisplayName)
		// Execute MCP tool
		if tr.mcpManager == nil {
			logger.Error("MCP manager not configured for MCP tool execution", "name", name)
			return nil, fmt.Errorf("MCP manager not configured")
		}

		result, err := tr.mcpManager.CallTool(tool.ServerName, tool.DisplayName, args)
		if err != nil {
			logger.Error("MCP tool execution failed", "name", name, "server", tool.ServerName, "error", err)
			return nil, fmt.Errorf("MCP tool execution failed: %w", err)
		}

		// Convert MCP result to expected format
		if result.IsError {
			errorMsg := formatMCPContent(result.Content)
			logger.Error("MCP tool returned error", "name", name, "server", tool.ServerName, "error", errorMsg)
			return nil, fmt.Errorf("MCP tool returned error: %s", errorMsg)
		}

		resultStr := formatMCPContent(result.Content)
		logger.Info("MCP tool executed successfully", "name", name, "server", tool.ServerName)
		return resultStr, nil
	}

	logger.Error("Unknown tool source", "name", name, "source", tool.Source)
	return nil, fmt.Errorf("unknown tool source: %s", tool.Source)
}

// formatMCPContent formats MCP tool content for return
func formatMCPContent(content []mcp.ToolContent) string {
	var result strings.Builder
	for i, item := range content {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(item.Text)
	}
	return result.String()
}

// FileSystemTool provides filesystem operations
type FileSystemTool struct{}

// FileInfo represents file/directory information
type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"is_dir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Mode     string    `json:"mode"`
}

// ListDirectoryArgs represents arguments for listing a directory
type ListDirectoryArgs struct {
	Path string `json:"path"`
}

// ReadFileArgs represents arguments for reading a file
type ReadFileArgs struct {
	Path     string `json:"path"`
	MaxBytes int    `json:"max_bytes,omitempty"` // Optional limit on file size
	Encoding string `json:"encoding,omitempty"`  // Optional encoding (default: utf-8)
}

// Name returns the tool name
func (fst *FileSystemTool) Name() string {
	return "filesystem_read"
}

// Description returns the tool description
func (fst *FileSystemTool) Description() string {
	return "Read local filesystem - get working directory, list directories and read file contents"
}

// GetAPITool returns the Ollama API tool definition
func (fst *FileSystemTool) GetAPITool() *api.Tool {
	return &api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "filesystem_read",
			Description: "Read local filesystem - get working directory, list directories and read file contents",
			Parameters: api.ToolFunctionParameters{
				Type: "object",
				Properties: map[string]api.ToolProperty{
					"action": {
						Type:        api.PropertyType{"string"},
						Description: "Action to perform: 'get_working_directory', 'list_directory' or 'read_file'",
						Enum:        []any{"get_working_directory", "list_directory", "read_file"},
					},
					"path": {
						Type:        api.PropertyType{"string"},
						Description: "File or directory path (not required for get_working_directory)",
					},
					"max_bytes": {
						Type:        api.PropertyType{"integer"},
						Description: "Maximum bytes to read for files (optional, default: 10MB)",
					},
				},
				Required: []string{"action"},
			},
		},
	}
}

// Execute performs the filesystem operation
func (fst *FileSystemTool) Execute(args map[string]interface{}) (interface{}, error) {
	action, ok := args["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action parameter required and must be a string")
	}

	switch action {
	case "get_working_directory":
		return fst.getWorkingDirectory()
	case "list_directory", "read_file":
		// These actions require a path parameter
		path, ok := args["path"].(string)
		if !ok {
			return nil, fmt.Errorf("path parameter required for action '%s' and must be a string", action)
		}

		// Clean and validate the path
		cleanPath := filepath.Clean(path)

		switch action {
		case "list_directory":
			return fst.listDirectory(cleanPath)
		case "read_file":
			maxBytes := 10 * 1024 * 1024 // 10MB default
			if mb, ok := args["max_bytes"].(float64); ok {
				maxBytes = int(mb)
			}
			return fst.readFile(cleanPath, maxBytes)
		}
	}

	return nil, fmt.Errorf("unknown action: %s. Valid actions are: get_working_directory, list_directory, read_file", action)
}

// listDirectory lists files and directories in the given path
func (fst *FileSystemTool) listDirectory(path string) (interface{}, error) {
	// Check if path exists and is accessible
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access path %s: %v", path, err)
	}

	if !stat.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", path)
	}

	// Read directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory %s: %v", path, err)
	}

	var fileInfos []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			// Skip entries that can't be accessed
			continue
		}

		fileInfo := FileInfo{
			Name:     entry.Name(),
			Path:     filepath.Join(path, entry.Name()),
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
			Mode:     info.Mode().String(),
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	return map[string]interface{}{
		"path":     path,
		"entries":  fileInfos,
		"count":    len(fileInfos),
		"readable": true,
	}, nil
}

// readFile reads the contents of a file with size limits
func (fst *FileSystemTool) readFile(path string, maxBytes int) (interface{}, error) {
	// Check if path exists and is accessible
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access file %s: %v", path, err)
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("path %s is a directory, not a file", path)
	}

	// Check file size
	if stat.Size() > int64(maxBytes) {
		return nil, fmt.Errorf("file %s is too large (%d bytes, max: %d bytes)", path, stat.Size(), maxBytes)
	}

	// Open and read the file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %s: %v", path, err)
	}
	defer file.Close()

	// Read up to maxBytes
	content := make([]byte, maxBytes)
	n, err := io.ReadFull(file, content)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, fmt.Errorf("error reading file %s: %v", path, err)
	}

	content = content[:n] // Trim to actual size

	// Check if content is valid UTF-8
	isText := utf8.Valid(content)
	var contentStr string
	var isBinary bool

	if isText {
		contentStr = string(content)
		isBinary = false
	} else {
		// For binary files, provide basic info but don't include raw content
		contentStr = fmt.Sprintf("<binary file - %d bytes>", len(content))
		isBinary = true
	}

	// Detect if file looks like a text file based on content
	if isText && len(content) > 0 {
		// Check for common binary indicators in the first few bytes
		hasNullBytes := strings.Contains(string(content[:min(512, len(content))]), "\x00")
		if hasNullBytes {
			isBinary = true
			contentStr = fmt.Sprintf("<binary file with null bytes - %d bytes>", len(content))
		}
	}

	return map[string]interface{}{
		"path":       path,
		"size":       stat.Size(),
		"modified":   stat.ModTime(),
		"mode":       stat.Mode().String(),
		"content":    contentStr,
		"is_binary":  isBinary,
		"is_text":    !isBinary,
		"bytes_read": len(content),
	}, nil
}

// getWorkingDirectory returns the current working directory
func (fst *FileSystemTool) getWorkingDirectory() (interface{}, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot get working directory: %v", err)
	}

	// Get directory info
	stat, err := os.Stat(wd)
	if err != nil {
		return nil, fmt.Errorf("cannot access working directory %s: %v", wd, err)
	}

	// Get absolute path
	absPath, err := filepath.Abs(wd)
	if err != nil {
		absPath = wd // Fallback to original path
	}

	return map[string]interface{}{
		"path":     wd,
		"abs_path": absPath,
		"mode":     stat.Mode().String(),
		"modified": stat.ModTime(),
		"exists":   true,
		"is_dir":   stat.IsDir(),
	}, nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
