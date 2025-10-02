package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestNormalizeID(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "int64 unchanged",
			input:    int64(123),
			expected: int64(123),
		},
		{
			name:     "float64 to int64",
			input:    float64(123),
			expected: int64(123),
		},
		{
			name:     "float64 with decimal to int64",
			input:    float64(123.0),
			expected: int64(123),
		},
		{
			name:     "int to int64",
			input:    int(123),
			expected: int64(123),
		},
		{
			name:     "string unchanged",
			input:    "test-id",
			expected: "test-id",
		},
		{
			name:     "nil unchanged",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeID(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeID(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestJSONRPCIDMatching(t *testing.T) {
	// Test that JSON unmarshaling preserves ID matching
	tests := []struct {
		name         string
		requestID    any
		responseJSON string
		shouldMatch  bool
	}{
		{
			name:         "int64 request matches float64 response",
			requestID:    int64(1),
			responseJSON: `{"jsonrpc":"2.0","id":1,"result":{}}`,
			shouldMatch:  true,
		},
		{
			name:         "string request matches string response",
			requestID:    "test-123",
			responseJSON: `{"jsonrpc":"2.0","id":"test-123","result":{}}`,
			shouldMatch:  true,
		},
		{
			name:         "mismatched IDs",
			requestID:    int64(1),
			responseJSON: `{"jsonrpc":"2.0","id":2,"result":{}}`,
			shouldMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the JSON response
			var response JSONRPCResponse
			err := json.Unmarshal([]byte(tt.responseJSON), &response)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			// Check if IDs match after normalization
			normalizedRequest := normalizeID(tt.requestID)
			normalizedResponse := normalizeID(response.ID)
			matches := normalizedRequest == normalizedResponse

			if matches != tt.shouldMatch {
				t.Errorf("ID matching: request=%v (%T), response=%v (%T), expected match=%v, got match=%v",
					tt.requestID, tt.requestID, response.ID, response.ID, tt.shouldMatch, matches)
			}
		})
	}
}

func TestClientResponseHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := configuration.MCPServer{
		Name:      "test-server",
		Command:   "echo",
		Arguments: []string{},
		Enabled:   true,
	}

	client := NewClient(ctx, server)

	// Test response handling without actually starting the server
	t.Run("handle response with matching ID", func(t *testing.T) {
		// Add a pending request
		respChan := make(chan *JSONRPCResponse, 1)
		client.reqMutex.Lock()
		client.pendingReqs[int64(1)] = respChan
		client.reqMutex.Unlock()

		// Create a response with float64 ID (as JSON would unmarshal)
		response := &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      float64(1), // JSON unmarshals numbers as float64
			Result:  map[string]any{"test": "value"},
		}

		// Handle the response
		client.handleResponse(response)

		// Check if response was delivered
		select {
		case receivedResponse := <-respChan:
			if receivedResponse.ID != float64(1) {
				t.Errorf("Expected response ID %v, got %v", float64(1), receivedResponse.ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Response not received within timeout")
		}

		// Clean up
		client.reqMutex.Lock()
		delete(client.pendingReqs, int64(1))
		client.reqMutex.Unlock()
		close(respChan)
	})

	t.Run("handle response with no matching ID", func(t *testing.T) {
		// Create a response with no matching pending request
		response := &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      float64(999), // No pending request with this ID
			Result:  map[string]any{"test": "value"},
		}

		// This should not panic or cause issues
		client.handleResponse(response)
		// Test passes if no panic occurs
	})
}

func TestMCPProtocolStructures(t *testing.T) {
	t.Run("InitializeResult with tools capability", func(t *testing.T) {
		// Test that we can properly unmarshal the server response that was causing issues
		responseJSON := `{
			"protocolVersion": "2024-11-05",
			"capabilities": {
				"logging": {},
				"tools": {"listChanged": true}
			},
			"serverInfo": {
				"name": "example-servers/everything",
				"version": "1.0.0"
			}
		}`

		var result InitializeResult
		err := json.Unmarshal([]byte(responseJSON), &result)
		if err != nil {
			t.Fatalf("Failed to unmarshal InitializeResult: %v", err)
		}

		if result.ProtocolVersion != "2024-11-05" {
			t.Errorf("Expected protocol version '2024-11-05', got '%s'", result.ProtocolVersion)
		}

		if result.ServerInfo.Name != "example-servers/everything" {
			t.Errorf("Expected server name 'example-servers/everything', got '%s'", result.ServerInfo.Name)
		}

		if result.Capabilities.Tools == nil {
			t.Error("Expected tools capability to be present")
		} else if !result.Capabilities.Tools.ListChanged {
			t.Error("Expected tools.listChanged to be true")
		}
	})
}
