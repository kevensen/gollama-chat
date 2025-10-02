package mcp

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCMessageParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected any
		wantErr  bool
	}{
		{
			name:  "valid request",
			input: `{"jsonrpc":"2.0","id":1,"method":"test","params":{"arg1":"value1"}}`,
			expected: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      float64(1), // JSON numbers are parsed as float64
				Method:  "test",
				Params:  map[string]any{"arg1": "value1"},
			},
			wantErr: false,
		},
		{
			name:  "valid response with result",
			input: `{"jsonrpc":"2.0","id":1,"result":{"success":true}}`,
			expected: &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      float64(1),
				Result:  map[string]any{"success": true},
			},
			wantErr: false,
		},
		{
			name:  "valid response with error",
			input: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}`,
			expected: &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      float64(1),
				Error: &JSONRPCError{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid notification",
			input: `{"jsonrpc":"2.0","method":"notification","params":{"data":"value"}}`,
			expected: &JSONRPCNotification{
				JSONRPC: "2.0",
				Method:  "notification",
				Params:  map[string]any{"data": "value"},
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseJSONRPCMessage([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Compare the results
			switch expected := tt.expected.(type) {
			case *JSONRPCRequest:
				req, ok := result.(*JSONRPCRequest)
				if !ok {
					t.Errorf("Expected JSONRPCRequest, got %T", result)
					return
				}
				if req.JSONRPC != expected.JSONRPC || req.Method != expected.Method {
					t.Errorf("Request mismatch: got %+v, want %+v", req, expected)
				}

			case *JSONRPCResponse:
				resp, ok := result.(*JSONRPCResponse)
				if !ok {
					t.Errorf("Expected JSONRPCResponse, got %T", result)
					return
				}
				if resp.JSONRPC != expected.JSONRPC {
					t.Errorf("Response JSONRPC mismatch: got %s, want %s", resp.JSONRPC, expected.JSONRPC)
				}
				if expected.Error != nil {
					if resp.Error == nil {
						t.Error("Expected error but got none")
					} else if resp.Error.Code != expected.Error.Code || resp.Error.Message != expected.Error.Message {
						t.Errorf("Error mismatch: got %+v, want %+v", resp.Error, expected.Error)
					}
				}

			case *JSONRPCNotification:
				notif, ok := result.(*JSONRPCNotification)
				if !ok {
					t.Errorf("Expected JSONRPCNotification, got %T", result)
					return
				}
				if notif.JSONRPC != expected.JSONRPC || notif.Method != expected.Method {
					t.Errorf("Notification mismatch: got %+v, want %+v", notif, expected)
				}
			}
		})
	}
}

func TestNewJSONRPCRequest(t *testing.T) {
	req := NewJSONRPCRequest(123, "test_method", map[string]string{"key": "value"})

	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC version 2.0, got %s", req.JSONRPC)
	}

	if req.ID != 123 {
		t.Errorf("Expected ID 123, got %v", req.ID)
	}

	if req.Method != "test_method" {
		t.Errorf("Expected method test_method, got %s", req.Method)
	}

	params, ok := req.Params.(map[string]string)
	if !ok {
		t.Errorf("Expected params to be map[string]string, got %T", req.Params)
	} else if params["key"] != "value" {
		t.Errorf("Expected params[key] to be 'value', got %s", params["key"])
	}
}

func TestNewJSONRPCResponse(t *testing.T) {
	result := map[string]bool{"success": true}
	resp := NewJSONRPCResponse(456, result)

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC version 2.0, got %s", resp.JSONRPC)
	}

	if resp.ID != 456 {
		t.Errorf("Expected ID 456, got %v", resp.ID)
	}

	if resp.Error != nil {
		t.Errorf("Expected no error, got %+v", resp.Error)
	}

	resultMap, ok := resp.Result.(map[string]bool)
	if !ok {
		t.Errorf("Expected result to be map[string]bool, got %T", resp.Result)
	} else if !resultMap["success"] {
		t.Errorf("Expected result[success] to be true, got %v", resultMap["success"])
	}
}

func TestNewJSONRPCError(t *testing.T) {
	errorResp := NewJSONRPCError(789, -32600, "Invalid Request", nil)

	if errorResp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC version 2.0, got %s", errorResp.JSONRPC)
	}

	if errorResp.ID != 789 {
		t.Errorf("Expected ID 789, got %v", errorResp.ID)
	}

	if errorResp.Result != nil {
		t.Errorf("Expected no result, got %+v", errorResp.Result)
	}

	if errorResp.Error == nil {
		t.Error("Expected error, got nil")
	} else {
		if errorResp.Error.Code != -32600 {
			t.Errorf("Expected error code -32600, got %d", errorResp.Error.Code)
		}
		if errorResp.Error.Message != "Invalid Request" {
			t.Errorf("Expected error message 'Invalid Request', got %s", errorResp.Error.Message)
		}
	}
}

func TestMCPProtocolSerialization(t *testing.T) {
	// Test serialization of MCP protocol structures
	t.Run("InitializeRequest", func(t *testing.T) {
		req := InitializeRequest{
			ProtocolVersion: MCPVersion,
			Capabilities: ClientCapabilities{
				Experimental: map[string]any{"test": true},
			},
			ClientInfo: ClientInfo{
				Name:    "test-client",
				Version: "1.0.0",
			},
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Errorf("Failed to marshal InitializeRequest: %v", err)
		}

		var unmarshaled InitializeRequest
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Errorf("Failed to unmarshal InitializeRequest: %v", err)
		}

		if unmarshaled.ProtocolVersion != req.ProtocolVersion {
			t.Errorf("ProtocolVersion mismatch: got %s, want %s", unmarshaled.ProtocolVersion, req.ProtocolVersion)
		}
		if unmarshaled.ClientInfo.Name != req.ClientInfo.Name {
			t.Errorf("ClientInfo.Name mismatch: got %s, want %s", unmarshaled.ClientInfo.Name, req.ClientInfo.Name)
		}
	})

	t.Run("Tool", func(t *testing.T) {
		tool := Tool{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]any{
					"input": map[string]any{
						"type":        "string",
						"description": "Input parameter",
					},
				},
				Required: []string{"input"},
			},
		}

		data, err := json.Marshal(tool)
		if err != nil {
			t.Errorf("Failed to marshal Tool: %v", err)
		}

		var unmarshaled Tool
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Errorf("Failed to unmarshal Tool: %v", err)
		}

		if unmarshaled.Name != tool.Name {
			t.Errorf("Name mismatch: got %s, want %s", unmarshaled.Name, tool.Name)
		}
		if unmarshaled.Description != tool.Description {
			t.Errorf("Description mismatch: got %s, want %s", unmarshaled.Description, tool.Description)
		}
		if len(unmarshaled.InputSchema.Required) != len(tool.InputSchema.Required) {
			t.Errorf("Required fields length mismatch: got %d, want %d", len(unmarshaled.InputSchema.Required), len(tool.InputSchema.Required))
		}
	})
}
