package models

import (
	"testing"
)

func TestIsEmbeddingModel(t *testing.T) {
	testCases := []struct {
		name           string
		modelName      string
		expectedResult bool
	}{
		{
			name:           "nomic embedding model",
			modelName:      "nomic-embed-text:latest",
			expectedResult: true,
		},
		{
			name:           "gemma embedding model",
			modelName:      "embeddinggemma:latest",
			expectedResult: true,
		},
		{
			name:           "bge embedding model",
			modelName:      "bge-large:latest",
			expectedResult: true,
		},
		{
			name:           "e5 embedding model",
			modelName:      "e5-large:latest",
			expectedResult: true,
		},
		{
			name:           "generic embedding model",
			modelName:      "text-embedding-ada:latest",
			expectedResult: true,
		},
		{
			name:           "sentence transformer model",
			modelName:      "sentence-transformers:latest",
			expectedResult: true,
		},
		{
			name:           "mpnet embedding model",
			modelName:      "all-mpnet-base-v2:latest",
			expectedResult: true,
		},
		{
			name:           "minilm embedding model",
			modelName:      "all-minilm-l6-v2:latest",
			expectedResult: true,
		},
		{
			name:           "chat model",
			modelName:      "llama3.3:latest",
			expectedResult: false,
		},
		{
			name:           "tool use model",
			modelName:      "llama3-groq-tool-use:8b",
			expectedResult: false,
		},
		{
			name:           "qwen model",
			modelName:      "qwen2.5:latest",
			expectedResult: false,
		},
		{
			name:           "mistral model",
			modelName:      "mistral:latest",
			expectedResult: false,
		},
		{
			name:           "instruct model with embed in name",
			modelName:      "embed-instruct-chat:latest",
			expectedResult: false,
		},
		{
			name:           "chat model with embed in name",
			modelName:      "embedded-chat-model:latest",
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isEmbeddingModel(tc.modelName, "")
			if result != tc.expectedResult {
				t.Errorf("Expected isEmbeddingModel(%q) to be %t, got %t", tc.modelName, tc.expectedResult, result)
			}
		})
	}
}

func TestFilterModels(t *testing.T) {
	testModels := []OllamaModel{
		{Name: "llama3.3:latest", Size: 1000000},
		{Name: "nomic-embed-text:latest", Size: 500000},
		{Name: "embeddinggemma:latest", Size: 600000},
		{Name: "mistral:latest", Size: 800000},
		{Name: "bge-large:latest", Size: 400000},
	}

	// Test embedding model filtering
	model := Model{mode: EmbeddingModelSelection}
	filtered := model.filterModels(testModels)

	expectedEmbeddingModels := 3 // nomic-embed-text, embeddinggemma, bge-large
	if len(filtered) != expectedEmbeddingModels {
		t.Errorf("Expected %d embedding models, got %d", expectedEmbeddingModels, len(filtered))
	}

	// Verify the correct models are included
	embeddingModelNames := make(map[string]bool)
	for _, model := range filtered {
		embeddingModelNames[model.Name] = true
	}

	expectedNames := []string{"nomic-embed-text:latest", "embeddinggemma:latest", "bge-large:latest"}
	for _, name := range expectedNames {
		if !embeddingModelNames[name] {
			t.Errorf("Expected embedding model %q to be in filtered results", name)
		}
	}

	// Test chat model filtering
	model.mode = ChatModelSelection
	filtered = model.filterModels(testModels)

	expectedChatModels := 2 // llama3.3, mistral
	if len(filtered) != expectedChatModels {
		t.Errorf("Expected %d chat models, got %d", expectedChatModels, len(filtered))
	}

	// Verify the correct models are included
	chatModelNames := make(map[string]bool)
	for _, model := range filtered {
		chatModelNames[model.Name] = true
	}

	expectedChatNames := []string{"llama3.3:latest", "mistral:latest"}
	for _, name := range expectedChatNames {
		if !chatModelNames[name] {
			t.Errorf("Expected chat model %q to be in filtered results", name)
		}
	}
}

func TestFilterModelsWithTextFilter(t *testing.T) {
	testModels := []OllamaModel{
		{Name: "llama3.3:latest", Size: 1000000},
		{Name: "llama3-groq:latest", Size: 1200000},
		{Name: "nomic-embed-text:latest", Size: 500000},
		{Name: "mistral:latest", Size: 800000},
	}

	// Test text filter with embedding mode
	model := Model{
		mode:   EmbeddingModelSelection,
		filter: "nomic",
	}
	filtered := model.filterModels(testModels)

	if len(filtered) != 1 {
		t.Errorf("Expected 1 model when filtering for 'nomic', got %d", len(filtered))
	}

	if len(filtered) > 0 && filtered[0].Name != "nomic-embed-text:latest" {
		t.Errorf("Expected 'nomic-embed-text:latest', got %q", filtered[0].Name)
	}

	// Test text filter with chat mode
	model.mode = ChatModelSelection
	model.filter = "llama"
	filtered = model.filterModels(testModels)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 models when filtering for 'llama', got %d", len(filtered))
	}
}
