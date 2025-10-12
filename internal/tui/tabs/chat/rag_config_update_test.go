package chat

import (
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

// TestUpdateFromConfiguration_RAGSettings tests that RAG service is properly updated when configuration changes
func TestUpdateFromConfiguration_RAGSettings(t *testing.T) {
	// Create initial configuration with RAG disabled
	initialConfig := &configuration.Config{
		ChatModel:           "test-model",
		EmbeddingModel:      "test-embedding",
		OllamaURL:           "http://localhost:11434",
		ChromaDBURL:         "http://localhost:8000",
		RAGEnabled:          false,
		SelectedCollections: map[string]bool{"collection1": true},
		MaxDocuments:        5,
		ChromaDBDistance:    1.0,
	}

	ctx := t.Context()
	model := NewModel(ctx, initialConfig)

	// Test 1: Enable RAG
	newConfig := &configuration.Config{
		ChatModel:           "test-model",
		EmbeddingModel:      "test-embedding",
		OllamaURL:           "http://localhost:11434",
		ChromaDBURL:         "http://localhost:8000",
		RAGEnabled:          true, // Changed from false to true
		SelectedCollections: map[string]bool{"collection1": true},
		MaxDocuments:        5,
		ChromaDBDistance:    1.0,
	}

	model.UpdateFromConfiguration(ctx, newConfig)

	// Verify configuration was updated
	if !model.config.RAGEnabled {
		t.Error("Expected RAG to be enabled after configuration update")
	}

	// Test 2: Change ChromaDB URL
	newConfig2 := &configuration.Config{
		ChatModel:           "test-model",
		EmbeddingModel:      "test-embedding",
		OllamaURL:           "http://localhost:11434",
		ChromaDBURL:         "http://localhost:9000", // Changed URL
		RAGEnabled:          true,
		SelectedCollections: map[string]bool{"collection1": true},
		MaxDocuments:        5,
		ChromaDBDistance:    1.0,
	}

	model.UpdateFromConfiguration(ctx, newConfig2)

	// Verify configuration was updated
	if model.config.ChromaDBURL != "http://localhost:9000" {
		t.Errorf("Expected ChromaDB URL to be updated to http://localhost:9000, got %s", model.config.ChromaDBURL)
	}

	// Test 3: Change selected collections
	newConfig3 := &configuration.Config{
		ChatModel:           "test-model",
		EmbeddingModel:      "test-embedding",
		OllamaURL:           "http://localhost:11434",
		ChromaDBURL:         "http://localhost:9000",
		RAGEnabled:          true,
		SelectedCollections: map[string]bool{"collection2": true, "collection3": true}, // Changed collections
		MaxDocuments:        5,
		ChromaDBDistance:    1.0,
	}

	model.UpdateFromConfiguration(ctx, newConfig3)

	// Verify configuration was updated
	if !model.config.SelectedCollections["collection2"] || !model.config.SelectedCollections["collection3"] {
		t.Error("Expected selected collections to be updated")
	}
	if model.config.SelectedCollections["collection1"] {
		t.Error("Expected collection1 to be unselected")
	}
}

// Test helper function equalStringMaps
func TestEqualStringMaps(t *testing.T) {
	tests := []struct {
		name     string
		map1     map[string]bool
		map2     map[string]bool
		expected bool
	}{
		{
			name:     "equal maps",
			map1:     map[string]bool{"a": true, "b": false},
			map2:     map[string]bool{"a": true, "b": false},
			expected: true,
		},
		{
			name:     "different values",
			map1:     map[string]bool{"a": true, "b": false},
			map2:     map[string]bool{"a": false, "b": false},
			expected: false,
		},
		{
			name:     "different keys",
			map1:     map[string]bool{"a": true, "b": false},
			map2:     map[string]bool{"a": true, "c": false},
			expected: false,
		},
		{
			name:     "different lengths",
			map1:     map[string]bool{"a": true},
			map2:     map[string]bool{"a": true, "b": false},
			expected: false,
		},
		{
			name:     "both empty",
			map1:     map[string]bool{},
			map2:     map[string]bool{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := equalStringMaps(tt.map1, tt.map2)
			if result != tt.expected {
				t.Errorf("equalStringMaps() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
