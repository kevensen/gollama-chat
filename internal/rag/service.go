package rag

import (
	"context"
	"fmt"
	"sort"
	"time"

	v2 "github.com/amikos-tech/chroma-go/pkg/api/v2"
	"github.com/amikos-tech/chroma-go/pkg/embeddings"
	"github.com/amikos-tech/chroma-go/pkg/embeddings/ollama"
	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/logging"
)

// RetrievedDocument represents a document retrieved from ChromaDB with relevance score
type RetrievedDocument struct {
	Content    string            `json:"content"`
	Metadata   map[string]string `json:"metadata"`
	Collection string            `json:"collection"`
	Distance   float32           `json:"distance"`
	ID         string            `json:"id"`
}

// RAGResult contains the retrieved documents and any errors
type RAGResult struct {
	Documents []RetrievedDocument `json:"documents"`
	Query     string              `json:"query"`
	Error     error               `json:"error,omitempty"`
}

// Service handles RAG operations using ChromaDB
type Service struct {
	config              *configuration.Config
	client              v2.Client
	embeddingFunc       embeddings.EmbeddingFunction
	connected           bool
	selectedCollections []string
}

// NewService creates a new RAG service
func NewService(config *configuration.Config) *Service {
	return &Service{
		config:              config,
		connected:           false,
		selectedCollections: make([]string, 0),
	}
}

// Initialize sets up the ChromaDB client and embedding function
func (s *Service) Initialize(ctx context.Context) error {
	logger := logging.WithComponent("rag")

	if s.config.ChromaDBURL == "" {
		return fmt.Errorf("ChromaDB URL not configured")
	}

	logger.Info("Initializing RAG service",
		"chromadb_url", s.config.ChromaDBURL,
		"ollama_url", s.config.OllamaURL,
		"embedding_model", s.config.EmbeddingModel,
	)

	// Create ChromaDB client
	logger.Info("Creating ChromaDB client", "chromadb_url", s.config.ChromaDBURL)
	client, err := v2.NewHTTPClient(v2.WithBaseURL(s.config.ChromaDBURL))
	if err != nil {
		logger.Error("Failed to create ChromaDB client", "error", err.Error())
		return fmt.Errorf("failed to create ChromaDB client: %w", err)
	}

	// Test connection
	logger.Info("Testing connection to ChromaDB data store")
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	collections, err := client.ListCollections(ctx)
	if err != nil {
		logger.Error("Failed to connect to ChromaDB data store",
			"chromadb_url", s.config.ChromaDBURL,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to connect to ChromaDB: %w", err)
	}

	logger.Info("Successfully connected to ChromaDB data store",
		"available_collections", len(collections),
		"chromadb_url", s.config.ChromaDBURL,
	)

	// Create Ollama embedding function
	logger.Info("Creating Ollama embedding function",
		"ollama_url", s.config.OllamaURL,
		"embedding_model", s.config.EmbeddingModel,
	)
	embeddingFunc, err := ollama.NewOllamaEmbeddingFunction(
		ollama.WithBaseURL(s.config.OllamaURL),
		ollama.WithModel(embeddings.EmbeddingModel(s.config.EmbeddingModel)),
	)
	if err != nil {
		logger.Error("Failed to create Ollama embedding function", "error", err.Error())
		return fmt.Errorf("failed to create Ollama embedding function: %w", err)
	}

	// Test the embedding function to ensure it works
	logger.Info("Testing embedding function with sample text")
	testCtx, testCancel := context.WithTimeout(ctx, 10*time.Second)
	defer testCancel()

	_, err = embeddingFunc.EmbedDocuments(testCtx, []string{"test"})
	if err != nil {
		logger.Error("Embedding model test failed - this model does not support embeddings",
			"embedding_model", s.config.EmbeddingModel,
			"error", err.Error(),
		)
		return fmt.Errorf("embedding model '%s' does not support embeddings. Please use a model like 'nomic-embed-text:latest' or 'all-minilm:latest': %w", s.config.EmbeddingModel, err)
	}
	logger.Info("Embedding function test successful")

	s.client = client
	s.embeddingFunc = embeddingFunc
	s.connected = true

	// Automatically load and select all collections if none are selected
	if len(s.selectedCollections) == 0 {
		logger.Info("No collections selected, auto-loading all available collections")

		collections, err := client.ListCollections(ctx)
		if err != nil {
			logger.Warn("Failed to auto-load collections for RAG service", "error", err.Error())
		} else {
			s.selectedCollections = make([]string, 0, len(collections))
			for _, collection := range collections {
				s.selectedCollections = append(s.selectedCollections, collection.Name())
			}
			logger.Info("Auto-selected all available collections for RAG service",
				"selected_collections", s.selectedCollections,
				"count", len(s.selectedCollections))
		}
	}

	logger.Info("RAG service initialization completed successfully")
	return nil
}

// UpdateSelectedCollections updates the list of selected collections
func (s *Service) UpdateSelectedCollections(ctx context.Context, selectedCollections map[string]bool) {
	logger := logging.WithComponent("rag")

	// If no collections are specifically selected (empty map), auto-select all available collections
	if len(selectedCollections) == 0 {
		logger.Info("No collections specified in configuration, attempting to auto-select all available collections")

		// Only auto-select if we have a connected client
		if s.connected && s.client != nil {
			collections, err := s.client.ListCollections(ctx)
			if err != nil {
				logger.Warn("Failed to auto-load collections, keeping existing selections", "error", err.Error())
				// Keep existing selections if we can't fetch collections
				return
			}

			s.selectedCollections = make([]string, 0, len(collections))
			for _, collection := range collections {
				s.selectedCollections = append(s.selectedCollections, collection.Name())
			}
			logger.Info("Auto-selected all available collections",
				"selected_collections", s.selectedCollections,
				"count", len(s.selectedCollections))
		} else {
			logger.Info("RAG service not connected, cannot auto-select collections")
			s.selectedCollections = make([]string, 0)
		}
	} else {
		// Use explicitly selected collections
		s.selectedCollections = make([]string, 0)
		for collection, selected := range selectedCollections {
			if selected {
				s.selectedCollections = append(s.selectedCollections, collection)
			}
		}
		logger.Info("Updated RAG service with explicitly selected collections",
			"selected_collections", s.selectedCollections,
			"count", len(s.selectedCollections))
	}
}

// GetSelectedCollections returns the list of currently selected collections
func (s *Service) GetSelectedCollections() []string {
	return s.selectedCollections
}

// UpdateConfig updates the service's configuration reference
func (s *Service) UpdateConfig(newConfig *configuration.Config) {
	logger := logging.WithComponent("rag")
	logger.Info("Updating RAG service configuration reference",
		"old_chromadb_url", s.config.ChromaDBURL,
		"new_chromadb_url", newConfig.ChromaDBURL,
		"old_embedding_model", s.config.EmbeddingModel,
		"new_embedding_model", newConfig.EmbeddingModel,
		"old_rag_enabled", s.config.RAGEnabled,
		"new_rag_enabled", newConfig.RAGEnabled,
	)
	s.config = newConfig
}

// IsReady checks if the service is ready to perform RAG operations
func (s *Service) IsReady() bool {
	logger := logging.WithComponent("rag")
	ready := s.config.RAGEnabled && s.connected && len(s.selectedCollections) > 0

	logger.Info("RAG service readiness check",
		"rag_enabled", s.config.RAGEnabled,
		"connected", s.connected,
		"selected_collections_count", len(s.selectedCollections),
		"selected_collections", s.selectedCollections,
		"is_ready", ready,
	)

	return ready
} // QueryDocuments retrieves relevant documents for the given query
func (s *Service) QueryDocuments(ctx context.Context, query string) (*RAGResult, error) {
	logger := logging.WithComponent("rag")

	if !s.connected {
		return nil, fmt.Errorf("RAG service not connected to ChromaDB")
	}

	if !s.config.RAGEnabled {
		return nil, fmt.Errorf("RAG is disabled in configuration")
	}

	if len(s.selectedCollections) == 0 {
		return nil, fmt.Errorf("no collections selected for RAG")
	}

	// Log the start of RAG query with selected collections
	logger.Info("Starting RAG query",
		"query_preview", contentPreview(query, 100),
		"selected_collections", s.selectedCollections,
		"max_documents", s.config.MaxDocuments,
		"distance_threshold", s.config.ChromaDBDistance,
	)

	result := &RAGResult{
		Query:     query,
		Documents: make([]RetrievedDocument, 0),
	}

	// Query each selected collection
	for _, collectionName := range s.selectedCollections {
		// Log accessing each collection
		logger.Info("Accessing collection",
			"collection_name", collectionName,
			"chromadb_url", s.config.ChromaDBURL,
		)

		docs, err := s.queryCollection(ctx, collectionName, query)
		if err != nil {
			// Log error but continue with other collections
			logger.Warn("Failed to query collection",
				"collection_name", collectionName,
				"error", err.Error(),
			)
			continue
		}

		// Log successful collection query
		logger.Info("Collection query completed",
			"collection_name", collectionName,
			"documents_found", len(docs),
		)

		result.Documents = append(result.Documents, docs...)
	}

	// Sort documents by distance (most relevant first)
	sort.Slice(result.Documents, func(i, j int) bool {
		return result.Documents[i].Distance < result.Documents[j].Distance
	})

	// Limit total documents to maxDocuments from config
	if len(result.Documents) > s.config.MaxDocuments {
		logger.Info("Limiting documents to max",
			"total_found", len(result.Documents),
			"max_documents", s.config.MaxDocuments,
		)
		result.Documents = result.Documents[:s.config.MaxDocuments]
	}

	// Log final results
	logger.Info("RAG query completed",
		"total_documents_returned", len(result.Documents),
		"collections_queried", len(s.selectedCollections),
	)

	return result, nil
}

// queryCollection queries a specific collection for relevant documents
func (s *Service) queryCollection(ctx context.Context, collectionName, query string) ([]RetrievedDocument, error) {
	logger := logging.WithComponent("rag")

	// Log attempting to get collection
	logger.Info("Getting collection from data store",
		"collection_name", collectionName,
		"chromadb_url", s.config.ChromaDBURL,
	)

	// Get the collection with embedding function
	collection, err := s.client.GetCollection(ctx, collectionName, v2.WithEmbeddingFunctionGet(s.embeddingFunc))
	if err != nil {
		logger.Warn("Failed to get collection",
			"collection_name", collectionName,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to get collection %s: %w", collectionName, err)
	}

	// Log successful collection access and querying
	logger.Info("Querying collection",
		"collection_name", collectionName,
		"max_results", s.config.MaxDocuments,
		"distance_threshold", s.config.ChromaDBDistance,
	)

	// Query the collection
	queryResult, err := collection.Query(
		ctx,
		v2.WithQueryTexts(query),
		v2.WithNResults(s.config.MaxDocuments),
		v2.WithIncludeQuery("documents", "metadatas", "distances"),
	)
	if err != nil {
		logger.Warn("Failed to query collection",
			"collection_name", collectionName,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to query collection %s: %w", collectionName, err)
	}

	documents := make([]RetrievedDocument, 0)
	totalResults := 0
	filteredResults := 0

	// Process query results
	for groupIdx, group := range queryResult.GetDocumentsGroups() {
		for i, doc := range group {
			totalResults++

			// Get distance for this document
			var distance float32 = 1.0 // Default high distance
			if distanceGroups := queryResult.GetDistancesGroups(); len(distanceGroups) > groupIdx && len(distanceGroups[groupIdx]) > i {
				distance = float32(distanceGroups[groupIdx][i])
			}

			// Filter by distance threshold
			if distance > float32(s.config.ChromaDBDistance) {
				continue
			}

			filteredResults++

			// Get metadata for this document
			metadata := make(map[string]string)
			if metadataGroups := queryResult.GetMetadatasGroups(); len(metadataGroups) > groupIdx && len(metadataGroups[groupIdx]) > i {
				docMetadata := metadataGroups[groupIdx][i]
				if docMetadata != nil {
					// Type assert to DocumentMetadataImpl to access Keys() method
					if impl, ok := docMetadata.(*v2.DocumentMetadataImpl); ok {
						for _, key := range impl.Keys() {
							if value, ok := impl.GetRaw(key); ok && value != nil {
								metadata[key] = fmt.Sprintf("%v", value)
							}
						}
					}
				}
			}

			// Get document ID
			var docID string
			if idGroups := queryResult.GetIDGroups(); len(idGroups) > groupIdx && len(idGroups[groupIdx]) > i {
				docID = string(idGroups[groupIdx][i])
			}

			documents = append(documents, RetrievedDocument{
				Content:    doc.ContentString(),
				Metadata:   metadata,
				Collection: collectionName,
				Distance:   distance,
				ID:         docID,
			})
		}
	}

	// Log detailed results for this collection
	logger.Info("Collection query results",
		"collection_name", collectionName,
		"total_results_returned", totalResults,
		"results_after_distance_filter", filteredResults,
		"distance_threshold", s.config.ChromaDBDistance,
		"relevant_documents", len(documents),
	)

	return documents, nil
}

// FormatDocumentsForPrompt formats retrieved documents for inclusion in chat prompt
func (r *RAGResult) FormatDocumentsForPrompt() string {
	if len(r.Documents) == 0 {
		return ""
	}

	var prompt string
	prompt += "=== RELEVANT CONTEXT ===\n"
	prompt += fmt.Sprintf("The following %d document(s) were retrieved to help answer your question:\n\n", len(r.Documents))

	for i, doc := range r.Documents {
		prompt += fmt.Sprintf("Document %d (Collection: %s, Relevance: %.3f):\n", i+1, doc.Collection, 1.0-doc.Distance)
		prompt += doc.Content + "\n\n"
	}

	prompt += "=== END CONTEXT ===\n\n"
	prompt += "Please use the above context to help answer the following question:\n"

	return prompt
}

// contentPreview returns a truncated preview of content for logging
func contentPreview(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "..."
}
