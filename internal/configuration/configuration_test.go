package configuration

import (
	"strings"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default config",
			config:      DefaultConfig(),
			expectError: false,
		},
		{
			name: "valid config with RAG disabled",
			config: &Config{
				ChatModel:           "llama3.3:latest",
				EmbeddingModel:      "embeddinggemma:latest",
				RAGEnabled:          false,
				OllamaURL:           "http://localhost:11434",
				ChromaDBURL:         "",
				ChromaDBDistance:    1.0,
				MaxDocuments:        5,
				SelectedCollections: make(map[string]bool),
				DefaultSystemPrompt: "You are a helpful assistant.",
				MCPServers:          []MCPServer{},
			},
			expectError: false,
		},
		{
			name: "valid config with MCP servers",
			config: &Config{
				ChatModel:           "llama3.3:latest",
				EmbeddingModel:      "embeddinggemma:latest",
				RAGEnabled:          true,
				OllamaURL:           "http://localhost:11434",
				ChromaDBURL:         "http://localhost:8000",
				ChromaDBDistance:    1.0,
				MaxDocuments:        5,
				SelectedCollections: make(map[string]bool),
				DefaultSystemPrompt: "You are a helpful assistant.",
				MCPServers: []MCPServer{
					{
						Name:      "test-server",
						Command:   "/path/to/server",
						Arguments: []string{"--arg1", "value1"},
						Enabled:   true,
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty ollama URL",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
			},
			expectError: true,
			errorMsg:    "ollamaURL cannot be empty",
		},
		{
			name: "MCP server with empty name",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
				MCPServers: []MCPServer{
					{
						Name:    "",
						Command: "/path/to/server",
						Enabled: true,
					},
				},
			},
			expectError: true,
			errorMsg:    "MCP server at index 0 has empty name",
		},
		{
			name: "MCP server with empty command",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
				MCPServers: []MCPServer{
					{
						Name:    "test-server",
						Command: "",
						Enabled: true,
					},
				},
			},
			expectError: true,
			errorMsg:    "MCP server 'test-server' has empty command",
		},
		{
			name: "duplicate MCP server names",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
				MCPServers: []MCPServer{
					{
						Name:    "test-server",
						Command: "/path/to/server1",
						Enabled: true,
					},
					{
						Name:    "test-server",
						Command: "/path/to/server2",
						Enabled: true,
					},
				},
			},
			expectError: true,
			errorMsg:    "duplicate MCP server name: test-server",
		},
		{
			name: "empty chat model",
			config: &Config{
				ChatModel:        "",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
			},
			expectError: true,
			errorMsg:    "chatModel cannot be empty",
		},
		{
			name: "ChromaDB distance too high",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 2.5,
				MaxDocuments:     5,
			},
			expectError: true,
			errorMsg:    "chromaDBDistance must be between 0 and 2 (cosine similarity range)",
		},
		// Additional comprehensive test cases
		{
			name: "empty embedding model when RAG disabled (should be valid)",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "",
				RAGEnabled:       false, // When RAG disabled, embedding model is not required
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
			},
			expectError: false,
		},
		{
			name: "empty embedding model when RAG enabled (should fail)",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "",
				RAGEnabled:       true, // When RAG enabled, embedding model is required
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
			},
			expectError: true,
			errorMsg:    "embeddingModel cannot be empty when RAG is enabled",
		},
		{
			name: "empty ChromaDB URL when RAG enabled",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
			},
			expectError: true,
			errorMsg:    "chromaDBURL cannot be empty when RAG is enabled",
		},
		{
			name: "empty ChromaDB URL when RAG disabled (should pass)",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       false,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
			},
			expectError: false,
		},
		{
			name: "negative ChromaDB distance",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: -0.5,
				MaxDocuments:     5,
			},
			expectError: true,
			errorMsg:    "chromaDBDistance must be between 0 and 2 (cosine similarity range)",
		},
		{
			name: "zero max documents",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     0,
			},
			expectError: true,
			errorMsg:    "maxDocuments must be greater than 0 when RAG is enabled",
		},
		{
			name: "negative max documents",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     -5,
			},
			expectError: true,
			errorMsg:    "maxDocuments must be greater than 0 when RAG is enabled",
		},
		{
			name: "boundary values - minimum valid",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 0.0, // Minimum valid distance
				MaxDocuments:     1,   // Minimum valid documents
			},
			expectError: false,
		},
		{
			name: "boundary values - maximum valid",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "embeddinggemma:latest",
				RAGEnabled:       true,
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 2.0,  // Maximum valid distance
				MaxDocuments:     1000, // Large but valid documents
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid, got error: %v", err)
	}

	tests := []struct {
		name     string
		actual   any
		expected any
	}{
		{"ChatModel", config.ChatModel, "llama3.3:latest"},
		{"EmbeddingModel", config.EmbeddingModel, "nomic-embed-text:latest"},
		{"RAGEnabled", config.RAGEnabled, false},
		{"OllamaURL", config.OllamaURL, "http://localhost:11434"},
		{"ChromaDBURL", config.ChromaDBURL, "http://localhost:8000"},
		{"ChromaDBDistance", config.ChromaDBDistance, 1.0},
		{"MaxDocuments", config.MaxDocuments, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("Expected %s to be %v, got %v", tt.name, tt.expected, tt.actual)
			}
		})
	}

	if config.SelectedCollections == nil {
		t.Error("SelectedCollections should be initialized")
	}

	if config.DefaultSystemPrompt == "" {
		t.Error("DefaultSystemPrompt should not be empty")
	}

	if config.MCPServers == nil {
		t.Error("MCPServers should be initialized")
	}

	if len(config.MCPServers) != 0 {
		t.Error("MCPServers should be empty by default")
	}
}

func TestConfig_MCPServerManagement(t *testing.T) {
	t.Run("GetMCPServer", func(t *testing.T) {
		config := DefaultConfig()
		config.MCPServers = []MCPServer{
			{Name: "server1", Command: "/path/to/server1", Enabled: true},
			{Name: "server2", Command: "/path/to/server2", Enabled: false},
		}

		// Test existing server
		server := config.GetMCPServer("server1")
		if server == nil {
			t.Error("Expected to find server1")
		} else if server.Name != "server1" || server.Command != "/path/to/server1" {
			t.Error("Got wrong server data")
		}

		// Test non-existing server
		server = config.GetMCPServer("nonexistent")
		if server != nil {
			t.Error("Expected nil for non-existent server")
		}
	})

	t.Run("AddMCPServer", func(t *testing.T) {
		config := DefaultConfig()

		// Test adding valid server
		server := MCPServer{
			Name:      "test-server",
			Command:   "/path/to/test",
			Arguments: []string{"--arg1", "value1"},
			Enabled:   true,
		}

		err := config.AddMCPServerNoSave(server)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if len(config.MCPServers) != 1 {
			t.Error("Expected 1 server after adding")
		}

		// Test adding duplicate server
		err = config.AddMCPServerNoSave(server)
		if err == nil {
			t.Error("Expected error when adding duplicate server")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("Expected 'already exists' error, got: %v", err)
		}
	})

	t.Run("UpdateMCPServer", func(t *testing.T) {
		config := DefaultConfig()
		config.MCPServers = []MCPServer{
			{Name: "server1", Command: "/path/to/server1", Enabled: true},
		}

		// Test updating existing server
		updatedServer := MCPServer{
			Name:      "server1",
			Command:   "/new/path/to/server1",
			Arguments: []string{"--new-arg"},
			Enabled:   false,
		}

		err := config.UpdateMCPServerNoSave("server1", updatedServer)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if config.MCPServers[0].Command != "/new/path/to/server1" {
			t.Error("Server command was not updated")
		}

		// Test updating non-existent server
		err = config.UpdateMCPServerNoSave("nonexistent", updatedServer)
		if err == nil {
			t.Error("Expected error when updating non-existent server")
		}
	})

	t.Run("RemoveMCPServer", func(t *testing.T) {
		config := DefaultConfig()
		config.MCPServers = []MCPServer{
			{Name: "server1", Command: "/path/to/server1", Enabled: true},
			{Name: "server2", Command: "/path/to/server2", Enabled: false},
		}

		// Test removing existing server
		err := config.RemoveMCPServerNoSave("server1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if len(config.MCPServers) != 1 {
			t.Error("Expected 1 server after removal")
		}

		if config.MCPServers[0].Name != "server2" {
			t.Error("Wrong server remained after removal")
		}

		// Test removing non-existent server
		err = config.RemoveMCPServerNoSave("nonexistent")
		if err == nil {
			t.Error("Expected error when removing non-existent server")
		}
	})

	t.Run("GetEnabledMCPServers", func(t *testing.T) {
		config := DefaultConfig()
		config.MCPServers = []MCPServer{
			{Name: "enabled1", Command: "/path/1", Enabled: true},
			{Name: "disabled1", Command: "/path/2", Enabled: false},
			{Name: "enabled2", Command: "/path/3", Enabled: true},
			{Name: "disabled2", Command: "/path/4", Enabled: false},
		}

		enabled := config.GetEnabledMCPServers()
		if len(enabled) != 2 {
			t.Errorf("Expected 2 enabled servers, got %d", len(enabled))
		}

		expectedNames := map[string]bool{"enabled1": true, "enabled2": true}
		for _, server := range enabled {
			if !expectedNames[server.Name] {
				t.Errorf("Unexpected enabled server: %s", server.Name)
			}
			if !server.Enabled {
				t.Errorf("Server %s should be enabled", server.Name)
			}
		}

		// Test with no enabled servers
		config.MCPServers = []MCPServer{
			{Name: "disabled1", Command: "/path/1", Enabled: false},
			{Name: "disabled2", Command: "/path/2", Enabled: false},
		}

		enabled = config.GetEnabledMCPServers()
		if len(enabled) != 0 {
			t.Errorf("Expected 0 enabled servers, got %d", len(enabled))
		}
	})
}

func TestDefaultConfig_Validation(t *testing.T) {
	// Test that the default configuration is valid and contains expected values
	t.Run("default config is valid", func(t *testing.T) {
		config := DefaultConfig()

		// Verify the configuration is valid
		if err := config.Validate(); err != nil {
			t.Errorf("Default configuration is invalid: %v", err)
		}

		// Verify critical values are set correctly
		if config.OllamaURL == "" {
			t.Error("Default OllamaURL should not be empty")
		}

		if config.ChatModel == "" {
			t.Error("Default ChatModel should not be empty")
		}

		// Verify the Ollama URL is the expected value for cold start
		expectedOllamaURL := "http://localhost:11434"
		if config.OllamaURL != expectedOllamaURL {
			t.Errorf("Expected default OllamaURL %s, got %s", expectedOllamaURL, config.OllamaURL)
		}
	})

	t.Run("default config has proper tool trust levels initialized", func(t *testing.T) {
		config := DefaultConfig()

		if config.ToolTrustLevels == nil {
			t.Error("ToolTrustLevels should be initialized")
		}

		// Should be empty but not nil
		if len(config.ToolTrustLevels) != 0 {
			t.Error("ToolTrustLevels should be empty by default")
		}
	})

	t.Run("default config has proper MCP servers initialized", func(t *testing.T) {
		config := DefaultConfig()

		if config.MCPServers == nil {
			t.Error("MCPServers should be initialized")
		}

		// Should be empty but not nil
		if len(config.MCPServers) != 0 {
			t.Error("MCPServers should be empty by default")
		}
	})
}
