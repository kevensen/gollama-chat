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
	if s.config.ChromaDBURL == "" {
		return fmt.Errorf("ChromaDB URL not configured")
	}

	// Create ChromaDB client
	client, err := v2.NewHTTPClient(v2.WithBaseURL(s.config.ChromaDBURL))
	if err != nil {
		return fmt.Errorf("failed to create ChromaDB client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to ChromaDB: %w", err)
	}

	// Create Ollama embedding function
	embeddingFunc, err := ollama.NewOllamaEmbeddingFunction(
		ollama.WithBaseURL(s.config.OllamaURL),
		ollama.WithModel(embeddings.EmbeddingModel(s.config.EmbeddingModel)),
	)
	if err != nil {
		return fmt.Errorf("failed to create Ollama embedding function: %w", err)
	}

	s.client = client
	s.embeddingFunc = embeddingFunc
	s.connected = true

	return nil
}

// UpdateSelectedCollections updates the list of selected collections
func (s *Service) UpdateSelectedCollections(selectedCollections map[string]bool) {
	s.selectedCollections = make([]string, 0)
	for collection, selected := range selectedCollections {
		if selected {
			s.selectedCollections = append(s.selectedCollections, collection)
		}
	}
}

// IsReady checks if the service is ready to perform RAG operations
func (s *Service) IsReady() bool {
	return s.config.RAGEnabled && s.connected && len(s.selectedCollections) > 0
}

// QueryDocuments retrieves relevant documents for the given query
func (s *Service) QueryDocuments(ctx context.Context, query string) (*RAGResult, error) {
	if !s.connected {
		return nil, fmt.Errorf("RAG service not connected to ChromaDB")
	}

	if !s.config.RAGEnabled {
		return nil, fmt.Errorf("RAG is disabled in configuration")
	}

	if len(s.selectedCollections) == 0 {
		return nil, fmt.Errorf("no collections selected for RAG")
	}

	result := &RAGResult{
		Query:     query,
		Documents: make([]RetrievedDocument, 0),
	}

	// Query each selected collection
	for _, collectionName := range s.selectedCollections {
		docs, err := s.queryCollection(ctx, collectionName, query)
		if err != nil {
			// Log error but continue with other collections
			continue
		}
		result.Documents = append(result.Documents, docs...)
	}

	// Sort documents by distance (most relevant first)
	sort.Slice(result.Documents, func(i, j int) bool {
		return result.Documents[i].Distance < result.Documents[j].Distance
	})

	// Limit total documents to maxDocuments from config
	if len(result.Documents) > s.config.MaxDocuments {
		result.Documents = result.Documents[:s.config.MaxDocuments]
	}

	return result, nil
}

// queryCollection queries a specific collection for relevant documents
func (s *Service) queryCollection(ctx context.Context, collectionName, query string) ([]RetrievedDocument, error) {
	// Get the collection
	collection, err := s.client.GetCollection(ctx, collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection %s: %w", collectionName, err)
	}

	// Query the collection
	queryResult, err := collection.Query(
		ctx,
		v2.WithQueryTexts(query),
		v2.WithNResults(s.config.MaxDocuments),
		v2.WithIncludeQuery("documents", "metadatas", "distances"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query collection %s: %w", collectionName, err)
	}

	documents := make([]RetrievedDocument, 0)

	// Process query results
	for groupIdx, group := range queryResult.GetDocumentsGroups() {
		for i, doc := range group {
			// Get distance for this document
			var distance float32 = 1.0 // Default high distance
			if distanceGroups := queryResult.GetDistancesGroups(); len(distanceGroups) > groupIdx && len(distanceGroups[groupIdx]) > i {
				distance = float32(distanceGroups[groupIdx][i])
			}

			// Filter by distance threshold
			if distance > float32(s.config.ChromaDBDistance) {
				continue
			}

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
