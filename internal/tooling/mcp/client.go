package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
)

// ServerStatus represents the current status of an MCP server
type ServerStatus int

const (
	StatusStopped ServerStatus = iota
	StatusStarting
	StatusRunning
	StatusError
)

func (s ServerStatus) String() string {
	switch s {
	case StatusStopped:
		return "Stopped"
	case StatusStarting:
		return "Starting"
	case StatusRunning:
		return "Running"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

// Client represents an MCP client that manages communication with an MCP server
type Client struct {
	server       configuration.MCPServer
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	status       ServerStatus
	statusMutex  sync.RWMutex
	requestID    int64
	pendingReqs  map[any]chan *JSONRPCResponse
	reqMutex     sync.RWMutex
	tools        []Tool
	toolsMutex   sync.RWMutex
	capabilities ServerCapabilities
	serverInfo   ServerInfo
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	lastError    error
}

// NewClient creates a new MCP client for the given server configuration
func NewClient(ctx context.Context, server configuration.MCPServer) *Client {
	ctx, cancel := context.WithCancel(ctx)
	return &Client{
		server:      server,
		status:      StatusStopped,
		pendingReqs: make(map[any]chan *JSONRPCResponse),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start initializes and starts the MCP server process
func (c *Client) Start() error {
	logger := logging.WithComponent("mcp-client")
	logger.Info("Starting MCP server", "server", c.server.Name, "command", c.server.Command, "args", c.server.Arguments)

	c.statusMutex.Lock()
	defer c.statusMutex.Unlock()

	if c.status != StatusStopped {
		err := fmt.Errorf("server is already running or starting")
		logger.Warn("Attempted to start already running server", "server", c.server.Name, "status", c.status.String())
		return err
	}

	c.status = StatusStarting
	logger.Debug("Server status changed to starting", "server", c.server.Name)

	// Create the command
	c.cmd = exec.CommandContext(c.ctx, c.server.Command, c.server.Arguments...)
	logger.Debug("Created command for MCP server", "server", c.server.Name, "command", c.server.Command, "args", c.server.Arguments)

	// Set up pipes
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		c.status = StatusError
		c.lastError = fmt.Errorf("failed to create stdin pipe: %w", err)
		logger.Error("Failed to create stdin pipe", "server", c.server.Name, "error", err)
		return c.lastError
	}
	c.stdin = stdin

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		c.status = StatusError
		c.lastError = fmt.Errorf("failed to create stdout pipe: %w", err)
		logger.Error("Failed to create stdout pipe", "server", c.server.Name, "error", err)
		return c.lastError
	}
	c.stdout = stdout

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		c.status = StatusError
		c.lastError = fmt.Errorf("failed to create stderr pipe: %w", err)
		logger.Error("Failed to create stderr pipe", "server", c.server.Name, "error", err)
		return c.lastError
	}
	c.stderr = stderr

	// Start the process
	logger.Debug("Starting MCP server process", "server", c.server.Name)
	if err := c.cmd.Start(); err != nil {
		c.status = StatusError
		c.lastError = fmt.Errorf("failed to start server process: %w", err)
		logger.Error("Failed to start MCP server process", "server", c.server.Name, "error", err)
		return c.lastError
	}
	logger.Info("MCP server process started successfully", "server", c.server.Name, "pid", c.cmd.Process.Pid)

	// Start goroutines for handling I/O
	c.wg.Add(3)
	go c.handleStdout()
	go c.handleStderr()
	go c.monitorProcess()

	// Initialize the MCP connection
	logger.Debug("Initializing MCP connection", "server", c.server.Name)
	if err := c.initialize(); err != nil {
		c.Stop()
		c.status = StatusError
		c.lastError = fmt.Errorf("failed to initialize MCP connection: %w", err)
		logger.Error("Failed to initialize MCP connection", "server", c.server.Name, "error", err)
		return c.lastError
	}

	c.status = StatusRunning
	logger.Info("MCP server started and initialized successfully", "server", c.server.Name)
	return nil
}

// Stop terminates the MCP server process and cleans up resources
func (c *Client) Stop() error {
	logger := logging.WithComponent("mcp-client")
	logger.Info("Stopping MCP server", "server", c.server.Name)

	// Check if already stopped and set status to stopping in a minimal critical section
	c.statusMutex.Lock()
	if c.status == StatusStopped {
		c.statusMutex.Unlock()
		logger.Debug("MCP server already stopped", "server", c.server.Name)
		return nil
	}
	// Set status to indicate stopping - this prevents GetStatus from blocking for long
	c.status = StatusStopped // Mark as stopped immediately to unblock any GetStatus calls
	c.statusMutex.Unlock()

	// Cancel the context to signal shutdown
	c.cancel()

	// Close stdin to signal the server to shutdown gracefully
	if c.stdin != nil {
		c.stdin.Close()
		logger.Debug("Closed stdin for graceful shutdown", "server", c.server.Name)
	}

	// Wait for the process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		if c.cmd != nil && c.cmd.Process != nil {
			done <- c.cmd.Wait()
		} else {
			done <- nil
		}
	}()

	select {
	case err := <-done:
		// Process exited naturally
		if err != nil {
			logger.Debug("MCP server process exited with error", "server", c.server.Name, "error", err)
		} else {
			logger.Debug("MCP server process exited cleanly", "server", c.server.Name)
		}
	case <-time.After(5 * time.Second):
		// Force kill after timeout
		logger.Warn("MCP server did not exit gracefully, force killing", "server", c.server.Name)
		if c.cmd != nil && c.cmd.Process != nil {
			c.cmd.Process.Kill()
		}
	}

	// Wait for all goroutines to finish
	c.wg.Wait()
	logger.Debug("All MCP server goroutines finished", "server", c.server.Name)

	// Clear pending requests
	c.reqMutex.Lock()
	for _, ch := range c.pendingReqs {
		close(ch)
	}
	c.pendingReqs = make(map[any]chan *JSONRPCResponse)
	c.reqMutex.Unlock()

	logger.Info("MCP server stopped successfully", "server", c.server.Name)
	return nil
}

// GetStatus returns the current status of the server
func (c *Client) GetStatus() ServerStatus {
	c.statusMutex.RLock()
	defer c.statusMutex.RUnlock()
	return c.status
}

// GetLastError returns the last error that occurred
func (c *Client) GetLastError() error {
	c.statusMutex.RLock()
	defer c.statusMutex.RUnlock()
	return c.lastError
}

// GetTools returns the list of tools provided by the server
func (c *Client) GetTools() []Tool {
	c.toolsMutex.RLock()
	defer c.toolsMutex.RUnlock()
	return append([]Tool(nil), c.tools...) // Return a copy
}

// CallTool invokes a tool on the MCP server
func (c *Client) CallTool(name string, arguments map[string]any) (*CallToolResult, error) {
	logger := logging.WithComponent("mcp-client")
	logger.Debug("Calling MCP tool", "server", c.server.Name, "tool", name, "arguments", arguments)

	if c.GetStatus() != StatusRunning {
		err := fmt.Errorf("server is not running")
		logger.Error("Cannot call tool - server not running", "server", c.server.Name, "tool", name, "status", c.GetStatus().String())
		return nil, err
	}

	req := CallToolRequest{
		Name:      name,
		Arguments: arguments,
	}

	var result CallToolResult
	if err := c.sendRequest("tools/call", req, &result); err != nil {
		logger.Error("Failed to call MCP tool", "server", c.server.Name, "tool", name, "error", err)
		return nil, fmt.Errorf("failed to call tool %s: %w", name, err)
	}

	logger.Info("Successfully called MCP tool", "server", c.server.Name, "tool", name, "isError", result.IsError)
	return &result, nil
}

// initialize performs the MCP initialization handshake
func (c *Client) initialize() error {
	logger := logging.WithComponent("mcp-client")
	logger.Debug("Starting MCP initialization handshake", "server", c.server.Name)

	initReq := InitializeRequest{
		ProtocolVersion: MCPVersion,
		Capabilities: ClientCapabilities{
			Experimental: make(map[string]any),
		},
		ClientInfo: ClientInfo{
			Name:    "gollama-chat",
			Version: "1.0.0",
		},
	}

	var initResult InitializeResult
	logger.Debug("Sending initialize request", "server", c.server.Name)
	if err := c.sendRequest("initialize", initReq, &initResult); err != nil {
		logger.Error("MCP initialization failed", "server", c.server.Name, "error", err)
		return fmt.Errorf("initialization failed: %w", err)
	}
	logger.Debug("Received initialize response", "server", c.server.Name, "serverInfo", initResult.ServerInfo.Name)

	c.capabilities = initResult.Capabilities
	c.serverInfo = initResult.ServerInfo

	// Send initialized notification
	notification := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	logger.Debug("Sending initialized notification", "server", c.server.Name)
	if err := c.sendNotification(&notification); err != nil {
		logger.Error("Failed to send initialized notification", "server", c.server.Name, "error", err)
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	// List available tools
	logger.Debug("Listing available tools", "server", c.server.Name)
	if err := c.refreshTools(); err != nil {
		logger.Error("Failed to list tools during initialization", "server", c.server.Name, "error", err)
		return fmt.Errorf("failed to list tools: %w", err)
	}

	logger.Info("MCP initialization completed successfully", "server", c.server.Name,
		"serverName", c.serverInfo.Name, "serverVersion", c.serverInfo.Version,
		"toolCount", len(c.tools))
	return nil
}

// refreshTools fetches the current list of tools from the server
func (c *Client) refreshTools() error {
	logger := logging.WithComponent("mcp-client")
	logger.Debug("Refreshing tools from MCP server", "server", c.server.Name)

	var result ListToolsResult
	if err := c.sendRequest("tools/list", ListToolsRequest{}, &result); err != nil {
		logger.Error("Failed to list tools from MCP server", "server", c.server.Name, "error", err)
		return fmt.Errorf("failed to list tools: %w", err)
	}

	c.toolsMutex.Lock()
	c.tools = result.Tools
	c.toolsMutex.Unlock()

	logger.Info("Successfully refreshed tools from MCP server", "server", c.server.Name, "toolCount", len(result.Tools))
	for _, tool := range result.Tools {
		logger.Debug("Available MCP tool", "server", c.server.Name, "toolName", tool.Name, "description", tool.Description)
	}

	return nil
}

// sendRequest sends a JSON-RPC request and waits for the response
func (c *Client) sendRequest(method string, params any, result any) error {
	logger := logging.WithComponent("mcp-client")
	id := atomic.AddInt64(&c.requestID, 1)
	logger.Debug("Sending JSON-RPC request", "server", c.server.Name, "method", method, "id", id)

	request := NewJSONRPCRequest(id, method, params)

	// Create response channel
	respChan := make(chan *JSONRPCResponse, 1)
	c.reqMutex.Lock()
	c.pendingReqs[id] = respChan
	c.reqMutex.Unlock()
	logger.Debug("Added pending request", "server", c.server.Name, "method", method, "id", id)

	// Clean up on exit
	defer func() {
		logger.Debug("Cleaning up pending request", "server", c.server.Name, "method", method, "id", id)
		c.reqMutex.Lock()
		delete(c.pendingReqs, id)
		c.reqMutex.Unlock()
		close(respChan)
	}()

	// Send the request
	data, err := json.Marshal(request)
	if err != nil {
		logger.Error("Failed to marshal JSON-RPC request", "server", c.server.Name, "method", method, "error", err)
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	logger.Debug("Writing JSON-RPC request to stdin", "server", c.server.Name, "method", method, "data", string(data))
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		logger.Error("Failed to write JSON-RPC request to stdin", "server", c.server.Name, "method", method, "error", err)
		return fmt.Errorf("failed to send request: %w", err)
	}

	logger.Debug("Waiting for JSON-RPC response", "server", c.server.Name, "method", method, "id", id)
	// Wait for response with timeout
	select {
	case response := <-respChan:
		logger.Debug("Received response from channel", "server", c.server.Name, "method", method, "id", id)
		if response == nil {
			logger.Error("JSON-RPC connection closed while waiting for response", "server", c.server.Name, "method", method, "id", id)
			return fmt.Errorf("connection closed")
		}
		if response.Error != nil {
			logger.Error("JSON-RPC server returned error", "server", c.server.Name, "method", method, "id", id, "error", response.Error.Message)
			return fmt.Errorf("server error: %s", response.Error.Message)
		}
		if result != nil {
			logger.Debug("Unmarshaling response result", "server", c.server.Name, "method", method, "id", id)
			data, err := json.Marshal(response.Result)
			if err != nil {
				logger.Error("Failed to marshal JSON-RPC response result", "server", c.server.Name, "method", method, "error", err)
				return fmt.Errorf("failed to marshal response result: %w", err)
			}
			if err := json.Unmarshal(data, result); err != nil {
				logger.Error("Failed to unmarshal JSON-RPC response result", "server", c.server.Name, "method", method, "error", err, "data", string(data))
				return fmt.Errorf("failed to unmarshal response result: %w", err)
			}
		}
		logger.Debug("Successfully received JSON-RPC response", "server", c.server.Name, "method", method, "id", id)
		return nil
	case <-time.After(30 * time.Second):
		logger.Error("JSON-RPC request timeout", "server", c.server.Name, "method", method, "id", id, "timeout", "30s")
		return fmt.Errorf("request timeout")
	case <-c.ctx.Done():
		logger.Debug("JSON-RPC request cancelled due to client shutdown", "server", c.server.Name, "method", method, "id", id)
		return fmt.Errorf("client shutting down")
	}
}

// sendNotification sends a JSON-RPC notification (no response expected)
func (c *Client) sendNotification(notification *JSONRPCNotification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// handleStdout processes messages from the server's stdout
func (c *Client) handleStdout() {
	logger := logging.WithComponent("mcp-client")
	defer c.wg.Done()
	scanner := bufio.NewScanner(c.stdout)

	for scanner.Scan() {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		logger.Debug("Received message from MCP server", "server", c.server.Name, "message", string(line))

		msg, err := ParseJSONRPCMessage(line)
		if err != nil {
			// Log error but continue processing
			logger.Error("Failed to parse JSON-RPC message from MCP server", "server", c.server.Name, "error", err, "message", string(line))
			continue
		}

		switch m := msg.(type) {
		case *JSONRPCResponse:
			logger.Debug("Handling JSON-RPC response", "server", c.server.Name, "id", m.ID)
			c.handleResponse(m)
		case *JSONRPCNotification:
			logger.Debug("Handling JSON-RPC notification", "server", c.server.Name, "method", m.Method)
			c.handleNotification(m)
		case *JSONRPCRequest:
			// We don't expect requests from the server in this implementation
			// but we could handle them here if needed
			logger.Debug("Received unexpected JSON-RPC request from MCP server", "server", c.server.Name, "method", m.Method)
		}
	}
}

// handleStderr processes error messages from the server
func (c *Client) handleStderr() {
	logger := logging.WithComponent("mcp-client")
	defer c.wg.Done()
	scanner := bufio.NewScanner(c.stderr)

	for scanner.Scan() {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Log stderr output from MCP server
		line := scanner.Text()
		if len(line) > 0 {
			logger.Warn("MCP server stderr", "server", c.server.Name, "message", line)
		}
	}
}

// monitorProcess monitors the server process and updates status
func (c *Client) monitorProcess() {
	logger := logging.WithComponent("mcp-client")
	defer c.wg.Done()

	if c.cmd == nil {
		logger.Error("No command to monitor for MCP server", "server", c.server.Name)
		return
	}

	logger.Debug("Starting to monitor MCP server process", "server", c.server.Name, "pid", c.cmd.Process.Pid)
	err := c.cmd.Wait()

	c.statusMutex.Lock()
	if c.status == StatusRunning {
		c.status = StatusError
		c.lastError = fmt.Errorf("server process exited unexpectedly: %v", err)
		logger.Error("MCP server process exited unexpectedly", "server", c.server.Name, "error", err)
	} else {
		logger.Debug("MCP server process exited as expected", "server", c.server.Name, "status", c.status.String())
	}
	c.statusMutex.Unlock()
}

// handleResponse routes responses to the appropriate waiting request
func (c *Client) handleResponse(response *JSONRPCResponse) {
	logger := logging.WithComponent("mcp-client")
	logger.Debug("Processing JSON-RPC response", "server", c.server.Name, "id", response.ID, "idType", fmt.Sprintf("%T", response.ID))

	c.reqMutex.RLock()
	defer c.reqMutex.RUnlock()

	// Find the matching pending request by normalizing ID comparison
	var respChan chan *JSONRPCResponse
	var foundKey any

	for key, ch := range c.pendingReqs {
		if normalizeID(key) == normalizeID(response.ID) {
			respChan = ch
			foundKey = key
			break
		}
	}

	if respChan != nil && foundKey != nil {
		logger.Debug("Found pending request channel, sending response", "server", c.server.Name, "requestID", foundKey, "responseID", response.ID)
		select {
		case respChan <- response:
			logger.Debug("Successfully sent response to channel", "server", c.server.Name, "id", response.ID)
		default:
			// Channel is full or closed, ignore
			logger.Warn("Failed to send response to channel (full or closed)", "server", c.server.Name, "id", response.ID)
		}
	} else {
		logger.Warn("No pending request found for response", "server", c.server.Name, "responseID", response.ID, "responseIDType", fmt.Sprintf("%T", response.ID), "pendingCount", len(c.pendingReqs))
		// Log all pending request IDs for debugging
		for key := range c.pendingReqs {
			logger.Debug("Pending request ID", "server", c.server.Name, "id", key, "idType", fmt.Sprintf("%T", key), "normalizedID", normalizeID(key))
		}
	}
}

// normalizeID converts different numeric ID types to a consistent format for comparison
func normalizeID(id any) any {
	switch v := id.(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case string:
		return v
	default:
		return id
	}
}

// handleNotification processes notifications from the server
func (c *Client) handleNotification(notification *JSONRPCNotification) {
	// Handle various notification types if needed
	// For now, we'll just ignore them
	_ = notification
}
