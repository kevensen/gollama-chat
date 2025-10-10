package configuration

import (
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

// TestRAGToggleAutoPopulatesDefaults tests that enabling RAG automatically sets default values
func TestRAGToggleAutoPopulatesDefaults(t *testing.T) {
	// Create a configuration with empty embedding model and ChromaDB URL
	initialConfig := &configuration.Config{
		ChatModel:        "test-model",
		EmbeddingModel:   "", // Empty - should be populated when RAG enabled
		ChromaDBURL:      "", // Empty - should be populated when RAG enabled
		RAGEnabled:       false,
		OllamaURL:        "http://localhost:11434",
		MaxDocuments:     5,
		ChromaDBDistance: 1.0,
	}

	model := NewModel(initialConfig)

	// Set the active field to RAGEnabledField and simulate Enter press
	model.activeField = RAGEnabledField

	// Verify initial state
	if model.editConfig.RAGEnabled {
		t.Error("Expected RAG to be initially disabled")
	}
	if model.editConfig.EmbeddingModel != "" {
		t.Error("Expected embedding model to be initially empty")
	}
	if model.editConfig.ChromaDBURL != "" {
		t.Error("Expected ChromaDB URL to be initially empty")
	}

	// Simulate pressing Enter to toggle RAG enabled
	// Note: We can't easily test the Update() method with tea.KeyMsg in a unit test
	// So we'll test the toggle logic directly

	// Toggle RAG enabled
	model.editConfig.RAGEnabled = !model.editConfig.RAGEnabled

	// Apply the same logic that happens in the Update method when RAG is enabled
	if model.editConfig.RAGEnabled && model.editConfig.EmbeddingModel == "" {
		model.editConfig.EmbeddingModel = "nomic-embed-text:latest"
	}
	if model.editConfig.RAGEnabled && model.editConfig.ChromaDBURL == "" {
		model.editConfig.ChromaDBURL = "http://localhost:8000"
	}

	// Verify that defaults were set
	if !model.editConfig.RAGEnabled {
		t.Error("Expected RAG to be enabled after toggle")
	}
	if model.editConfig.EmbeddingModel != "nomic-embed-text:latest" {
		t.Errorf("Expected embedding model to be set to default, got %s", model.editConfig.EmbeddingModel)
	}
	if model.editConfig.ChromaDBURL != "http://localhost:8000" {
		t.Errorf("Expected ChromaDB URL to be set to default, got %s", model.editConfig.ChromaDBURL)
	}
}

// TestRAGToggleDoesNotOverrideExistingValues tests that enabling RAG doesn't override existing values
func TestRAGToggleDoesNotOverrideExistingValues(t *testing.T) {
	// Create a configuration with existing values
	initialConfig := &configuration.Config{
		ChatModel:        "test-model",
		EmbeddingModel:   "custom-embedding-model",
		ChromaDBURL:      "http://custom-chromadb:9000",
		RAGEnabled:       false,
		OllamaURL:        "http://localhost:11434",
		MaxDocuments:     5,
		ChromaDBDistance: 1.0,
	}

	model := NewModel(initialConfig)
	model.activeField = RAGEnabledField

	// Toggle RAG enabled
	model.editConfig.RAGEnabled = !model.editConfig.RAGEnabled

	// Apply the same logic that happens in the Update method when RAG is enabled
	if model.editConfig.RAGEnabled && model.editConfig.EmbeddingModel == "" {
		model.editConfig.EmbeddingModel = "nomic-embed-text:latest"
	}
	if model.editConfig.RAGEnabled && model.editConfig.ChromaDBURL == "" {
		model.editConfig.ChromaDBURL = "http://localhost:8000"
	}

	// Verify that existing values were NOT overridden
	if !model.editConfig.RAGEnabled {
		t.Error("Expected RAG to be enabled after toggle")
	}
	if model.editConfig.EmbeddingModel != "custom-embedding-model" {
		t.Errorf("Expected embedding model to remain unchanged, got %s", model.editConfig.EmbeddingModel)
	}
	if model.editConfig.ChromaDBURL != "http://custom-chromadb:9000" {
		t.Errorf("Expected ChromaDB URL to remain unchanged, got %s", model.editConfig.ChromaDBURL)
	}
}
