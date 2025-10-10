package rag

import (
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestService_IsReady(t *testing.T) {
	tests := []struct {
		name                string
		ragEnabled          bool
		connected           bool
		selectedCollections []string
		expected            bool
	}{
		{
			name:                "ready - all conditions met",
			ragEnabled:          true,
			connected:           true,
			selectedCollections: []string{"collection1", "collection2"},
			expected:            true,
		},
		{
			name:                "not ready - RAG disabled",
			ragEnabled:          false,
			connected:           true,
			selectedCollections: []string{"collection1"},
			expected:            false,
		},
		{
			name:                "not ready - not connected",
			ragEnabled:          true,
			connected:           false,
			selectedCollections: []string{"collection1"},
			expected:            false,
		},
		{
			name:                "not ready - no collections selected",
			ragEnabled:          true,
			connected:           true,
			selectedCollections: []string{},
			expected:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &configuration.Config{
				RAGEnabled: tt.ragEnabled,
			}

			service := NewService(config)
			service.connected = tt.connected
			service.selectedCollections = tt.selectedCollections

			result := service.IsReady()
			if result != tt.expected {
				t.Errorf("IsReady() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestNewService(t *testing.T) {
	config := configuration.DefaultConfig()
	service := NewService(config)

	if service == nil {
		t.Fatal("NewService returned nil")
	}

	if service.config != config {
		t.Error("Service config should reference the provided config")
	}

	if service.connected {
		t.Error("New service should not be connected initially")
	}

	if service.selectedCollections == nil {
		t.Error("selectedCollections should be initialized")
	}

	if len(service.selectedCollections) != 0 {
		t.Error("selectedCollections should be empty initially")
	}
}

func TestService_UpdateConfig(t *testing.T) {
	// Create initial config
	initialConfig := &configuration.Config{
		RAGEnabled:       false,
		ChromaDBURL:      "http://localhost:8000",
		EmbeddingModel:   "old-model",
		MaxDocuments:     5,
		ChromaDBDistance: 1.0,
	}

	service := NewService(initialConfig)

	// Verify initial state
	if service.config != initialConfig {
		t.Error("Service should initially reference the initial config")
	}

	// Create new config with different values
	newConfig := &configuration.Config{
		RAGEnabled:       true,
		ChromaDBURL:      "http://new-chromadb:9000",
		EmbeddingModel:   "new-model",
		MaxDocuments:     10,
		ChromaDBDistance: 0.8,
	}

	// Update the config
	service.UpdateConfig(newConfig)

	// Verify the config was updated
	if service.config != newConfig {
		t.Error("Service should reference the new config after UpdateConfig")
	}

	if service.config.ChromaDBURL != "http://new-chromadb:9000" {
		t.Errorf("Expected ChromaDB URL to be updated to %s, got %s",
			"http://new-chromadb:9000", service.config.ChromaDBURL)
	}

	if service.config.EmbeddingModel != "new-model" {
		t.Errorf("Expected embedding model to be updated to %s, got %s",
			"new-model", service.config.EmbeddingModel)
	}

	if !service.config.RAGEnabled {
		t.Error("Expected RAG to be enabled in new config")
	}
}

func TestService_UpdateSelectedCollections_AutoSelect(t *testing.T) {
	config := &configuration.Config{
		RAGEnabled: true,
	}
	service := NewService(config)

	// Simulate a connected service (normally done by Initialize)
	service.connected = true
	// Note: We can't easily test the auto-selection without a real ChromaDB client
	// So we'll test the behavior when not connected

	// Test 1: Empty map with not connected service should clear collections
	service.selectedCollections = []string{"existing-collection"}
	service.connected = false

	emptyMap := make(map[string]bool)
	service.UpdateSelectedCollections(emptyMap)

	if len(service.selectedCollections) != 0 {
		t.Error("Expected collections to be cleared when not connected and empty map provided")
	}

	// Test 2: Non-empty map should use explicit selections
	explicitMap := map[string]bool{
		"collection1": true,
		"collection2": false,
		"collection3": true,
	}

	service.UpdateSelectedCollections(explicitMap)

	if len(service.selectedCollections) != 2 {
		t.Errorf("Expected 2 collections to be selected, got %d", len(service.selectedCollections))
	}

	// Check that only the true collections are selected
	expectedCollections := map[string]bool{
		"collection1": false,
		"collection3": false,
	}

	for _, collection := range service.selectedCollections {
		if _, exists := expectedCollections[collection]; !exists {
			t.Errorf("Unexpected collection selected: %s", collection)
		} else {
			expectedCollections[collection] = true
		}
	}

	for collection, found := range expectedCollections {
		if !found {
			t.Errorf("Expected collection %s to be selected but it wasn't", collection)
		}
	}
}
