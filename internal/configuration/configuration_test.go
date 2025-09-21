package configuration

import (
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
			name: "empty embedding model (always required)",
			config: &Config{
				ChatModel:        "llama3.3:latest",
				EmbeddingModel:   "",
				RAGEnabled:       false, // Even when RAG disabled, embedding model is required
				OllamaURL:        "http://localhost:11434",
				ChromaDBURL:      "http://localhost:8000",
				ChromaDBDistance: 1.0,
				MaxDocuments:     5,
			},
			expectError: true,
			errorMsg:    "embeddingModel cannot be empty",
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
			errorMsg:    "maxDocuments must be greater than 0",
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
			errorMsg:    "maxDocuments must be greater than 0",
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
		actual   interface{}
		expected interface{}
	}{
		{"ChatModel", config.ChatModel, "llama3.3:latest"},
		{"EmbeddingModel", config.EmbeddingModel, "embeddinggemma:latest"},
		{"RAGEnabled", config.RAGEnabled, true},
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
}
