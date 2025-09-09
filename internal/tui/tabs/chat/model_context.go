package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Cache for model context sizes to avoid repeated API calls
var contextSizeCache = make(map[string]int)
var contextSizeCacheMutex sync.RWMutex

// OllamaModelInfo represents the model_info section of the show API response
type OllamaModelInfo struct {
	ContextLength        int `json:"llama.context_length"`
	GeneralContextLength int `json:"general.context_length"`
}

// OllamaShowResponse represents the response from /api/show
type OllamaShowResponse struct {
	ModelInfo OllamaModelInfo `json:"model_info"`
}

// getModelContextSizeFromAPI fetches the context window size from the Ollama API
func getModelContextSizeFromAPI(modelName string, ollamaURL string) (int, error) {
	// Check cache first
	cacheKey := modelName + "@" + ollamaURL
	contextSizeCacheMutex.RLock()
	if size, found := contextSizeCache[cacheKey]; found {
		contextSizeCacheMutex.RUnlock()
		return size, nil
	}
	contextSizeCacheMutex.RUnlock()

	// Prepare the API request
	payload := map[string]interface{}{
		"model": modelName,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	// Make the API request
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(ollamaURL+"/api/show", "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Parse the response
	var response OllamaShowResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, err
	}

	// Get context length from response
	contextSize := 0
	if response.ModelInfo.ContextLength > 0 {
		contextSize = response.ModelInfo.ContextLength
	} else if response.ModelInfo.GeneralContextLength > 0 {
		contextSize = response.ModelInfo.GeneralContextLength
	} else {
		return 0, fmt.Errorf("no context length found in model info")
	}

	// Cache the result
	if contextSize > 0 {
		contextSizeCacheMutex.Lock()
		contextSizeCache[cacheKey] = contextSize
		contextSizeCacheMutex.Unlock()
	}

	return contextSize, nil
}

// getFallbackContextSize returns a context size from the hardcoded map or a default value
func getFallbackContextSize(modelName string) int {
	// Check if we have an exact match in our map
	if size, ok := modelContextSizes[modelName]; ok {
		return size
	}

	// If not, try to find a partial match by removing version suffixes
	for model, size := range modelContextSizes {
		// If the model name contains our model (ignoring versions)
		if strings.Contains(modelName, model) {
			return size
		}
	}

	// If we still don't have a match, try just the base name
	baseName := strings.Split(modelName, ":")[0]
	baseName = strings.Split(baseName, "-")[0]

	for model, size := range modelContextSizes {
		if strings.HasPrefix(model, baseName) {
			return size
		}
	}

	// Default to a common context size if we can't determine it
	return 8192 // 8K context is a safe default
}

// getModelContextSize returns the context window size for the given model
func (m Model) getModelContextSize(modelName string) int {
	// Try to get context size from the Ollama API first
	if m.config != nil && m.config.OllamaURL != "" {
		size, err := getModelContextSizeFromAPI(modelName, m.config.OllamaURL)
		if err == nil && size > 0 {
			return size
		}
		// If there's an error or size is 0, fall back to hardcoded values
	}

	// Fall back to hardcoded values
	return getFallbackContextSize(modelName)
}
