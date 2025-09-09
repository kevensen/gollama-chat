package rag

import (
	"context"
	"fmt"
	"net/http"
	"time"

	v2 "github.com/amikos-tech/chroma-go/pkg/api/v2"
	"github.com/kevensen/gollama-chat/internal/configuration"
)

// Collection represents a ChromaDB collection with selection state
type Collection struct {
	Name     string            `json:"name"`
	ID       string            `json:"id"`
	Metadata map[string]string `json:"metadata"`
	Selected bool              `json:"selected"`
}

// CollectionsService handles ChromaDB collections operations
type CollectionsService struct {
	Config      *configuration.Config
	client      v2.Client
	collections []Collection
	connected   bool
}

// NewCollectionsService creates a new collections service
func NewCollectionsService(config *configuration.Config) *CollectionsService {
	return &CollectionsService{
		Config:      config,
		collections: make([]Collection, 0),
		connected:   false,
	}
}

// TestConnection tests the ChromaDB connection
func (cs *CollectionsService) TestConnection() error {
	if cs.Config.ChromaDBURL == "" {
		cs.connected = false
		return fmt.Errorf("ChromaDB URL not configured")
	}

	// Use the same approach as the settings tab - simple HTTP GET to /api/v2
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(cs.Config.ChromaDBURL + "/api/v2")
	if err != nil {
		cs.connected = false
		return fmt.Errorf("failed to connect to ChromaDB: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cs.connected = false
		return fmt.Errorf("ChromaDB returned HTTP %d", resp.StatusCode)
	}

	// If basic connection works, try to create the full client for later use
	chromaClient, err := v2.NewHTTPClient(v2.WithBaseURL(cs.Config.ChromaDBURL))
	if err != nil {
		cs.connected = false
		return fmt.Errorf("failed to create ChromaDB client: %w", err)
	}

	cs.client = chromaClient
	cs.connected = true
	return nil
}

// LoadCollections loads collections from ChromaDB
func (cs *CollectionsService) LoadCollections(ctx context.Context) error {
	if !cs.connected || cs.client == nil {
		return fmt.Errorf("not connected to ChromaDB")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	chromaCollections, err := cs.client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	// Convert to our Collection format and select all by default
	cs.collections = make([]Collection, len(chromaCollections))
	for i, collection := range chromaCollections {
		// Convert metadata from chroma-go format to our expected format
		metadataMap := make(map[string]string)
		if metadata := collection.Metadata(); metadata != nil {
			for _, key := range metadata.Keys() {
				if strValue, ok := metadata.GetString(key); ok {
					metadataMap[key] = strValue
				} else if rawValue, ok := metadata.GetRaw(key); ok {
					metadataMap[key] = fmt.Sprintf("%v", rawValue)
				}
			}
		}

		cs.collections[i] = Collection{
			Name:     collection.Name(),
			ID:       collection.ID(),
			Metadata: metadataMap,
			Selected: true, // Select all by default as requested
		}
	}

	return nil
}

// GetCollections returns the list of collections
func (cs *CollectionsService) GetCollections() []Collection {
	return cs.collections
}

// IsConnected returns the connection status
func (cs *CollectionsService) IsConnected() bool {
	return cs.connected
}

// ToggleCollection toggles the selection state of a collection
func (cs *CollectionsService) ToggleCollection(index int) {
	if index >= 0 && index < len(cs.collections) {
		cs.collections[index].Selected = !cs.collections[index].Selected
	}
}

// SelectAll selects all collections
func (cs *CollectionsService) SelectAll() {
	for i := range cs.collections {
		cs.collections[i].Selected = true
	}
}

// DeselectAll deselects all collections
func (cs *CollectionsService) DeselectAll() {
	for i := range cs.collections {
		cs.collections[i].Selected = false
	}
}

// GetSelectedCollections returns a list of selected collection names
func (cs *CollectionsService) GetSelectedCollections() []string {
	var selected []string
	for _, collection := range cs.collections {
		if collection.Selected {
			selected = append(selected, collection.Name)
		}
	}
	return selected
}

// GetSelectedCount returns the number of selected collections
func (cs *CollectionsService) GetSelectedCount() int {
	count := 0
	for _, collection := range cs.collections {
		if collection.Selected {
			count++
		}
	}
	return count
}

// Close closes the ChromaDB client connection
func (cs *CollectionsService) Close() error {
	if cs.client != nil {
		return cs.client.Close()
	}
	return nil
}
