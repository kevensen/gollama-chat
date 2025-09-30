package configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Config represents the application configuration
type Config struct {
	ChatModel           string          `json:"chatModel"`
	EmbeddingModel      string          `json:"embeddingModel"`
	RAGEnabled          bool            `json:"ragEnabled"`
	OllamaURL           string          `json:"ollamaURL"`
	ChromaDBURL         string          `json:"chromaDBURL"`
	ChromaDBDistance    float64         `json:"chromaDBDistance"`
	MaxDocuments        int             `json:"maxDocuments"`
	SelectedCollections map[string]bool `json:"selectedCollections"`
	DefaultSystemPrompt string          `json:"defaultSystemPrompt"`
	ToolTrustLevels     map[string]int  `json:"toolTrustLevels"` // Maps tool name to trust level: 0=None(block), 1=Ask(prompt), 2=Session(allow)
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		ChatModel:           "llama3.3:latest",
		EmbeddingModel:      "embeddinggemma:latest",
		RAGEnabled:          true,
		OllamaURL:           "http://localhost:11434",
		ChromaDBURL:         "http://localhost:8000",
		ChromaDBDistance:    1.0, // Updated for cosine similarity (0-2 range)
		MaxDocuments:        5,
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     make(map[string]int),
		DefaultSystemPrompt: "You are a helpful Q&A bot. Your purpose is to provide direct, accurate answers to user questions. When providing lists of items (such as countries, capitals, features, etc.), format your response using proper numbered or bulleted lists. Be consistent in your formatting. If you don't know the answer, state that you are unable to provide a response.",
	}
}

// getConfigDir returns the appropriate config directory based on OS
func getConfigDir() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		configDir = os.Getenv("APPDATA")
		if configDir == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
	default: // Linux, macOS, and other Unix-like systems
		configDir = os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get user home directory: %w", err)
			}
			configDir = filepath.Join(homeDir, ".config")
		}
	}

	return filepath.Join(configDir, "gollama"), nil
}

// getConfigPath returns the full path to the configuration file
func getConfigPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "settings.json"), nil
}

// Load reads the configuration from the settings file
// If the file doesn't exist, it returns the default configuration
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply default values for any missing fields (needed for backward compatibility)
	applyDefaultsIfMissing(&config)

	return &config, nil
}

// applyDefaultsIfMissing sets default values for any config fields that might be missing
// This ensures backward compatibility when new fields are added
func applyDefaultsIfMissing(c *Config) {
	defaultConfig := DefaultConfig()

	// Check for empty DefaultSystemPrompt and apply default if needed
	if c.DefaultSystemPrompt == "" {
		c.DefaultSystemPrompt = defaultConfig.DefaultSystemPrompt
	}

	// Initialize ToolTrustLevels if nil (for backward compatibility)
	if c.ToolTrustLevels == nil {
		c.ToolTrustLevels = make(map[string]int)
	}

	// Add checks for any future fields here
}

// Save writes the configuration to the settings file
func (c *Config) Save() error {
	configDir, err := getConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.OllamaURL == "" {
		return fmt.Errorf("ollamaURL cannot be empty")
	}
	if c.ChatModel == "" {
		return fmt.Errorf("chatModel cannot be empty")
	}
	if c.EmbeddingModel == "" {
		return fmt.Errorf("embeddingModel cannot be empty")
	}
	if c.ChromaDBURL == "" && c.RAGEnabled {
		return fmt.Errorf("chromaDBURL cannot be empty when RAG is enabled")
	}
	if c.ChromaDBDistance < 0 || c.ChromaDBDistance > 2 {
		return fmt.Errorf("chromaDBDistance must be between 0 and 2 (cosine similarity range)")
	}
	if c.MaxDocuments <= 0 {
		return fmt.Errorf("maxDocuments must be greater than 0")
	}
	return nil
}

// GetToolTrustLevel returns the trust level for a tool, defaulting to 0 if not found
func (c *Config) GetToolTrustLevel(toolName string) int {
	if c.ToolTrustLevels == nil {
		return 0
	}
	return c.ToolTrustLevels[toolName]
}

// SetToolTrustLevel sets the trust level for a tool and saves the configuration
func (c *Config) SetToolTrustLevel(toolName string, trustLevel int) error {
	if c.ToolTrustLevels == nil {
		c.ToolTrustLevels = make(map[string]int)
	}
	c.ToolTrustLevels[toolName] = trustLevel
	return c.Save()
}
