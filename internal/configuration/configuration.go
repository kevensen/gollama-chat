package configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// MCPServer represents configuration for an MCP server
type MCPServer struct {
	Name      string   `json:"name"`      // Unique name for the server
	Command   string   `json:"command"`   // Path to the MCP server binary
	Arguments []string `json:"arguments"` // Arguments to pass to the server
	Enabled   bool     `json:"enabled"`   // Whether the server should be started
}

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
	ToolTrustLevels     map[string]int  `json:"toolTrustLevels"`   // Maps tool name to trust level: 0=None(block), 1=Ask(prompt), 2=Session(allow)
	MCPServers          []MCPServer     `json:"mcpServers"`        // MCP server configurations
	LogLevel            string          `json:"logLevel"`          // Log level: debug, info, warn, error
	EnableFileLogging   bool            `json:"enableFileLogging"` // Whether to log to file
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		ChatModel:           "llama3.3:latest",
		EmbeddingModel:      "",
		RAGEnabled:          false,
		OllamaURL:           "http://localhost:11434",
		ChromaDBURL:         "http://localhost:8000",
		ChromaDBDistance:    1.0, // Updated for cosine similarity (0-2 range)
		MaxDocuments:        5,
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     make(map[string]int),
		MCPServers:          []MCPServer{},
		LogLevel:            "info",
		EnableFileLogging:   true,
		DefaultSystemPrompt: "You are a helpful Q&A bot. Your purpose is to provide direct, accurate answers to user questions. When providing lists of items (such as countries, capitals, features, etc.), format your response using proper numbered or bulleted lists. Be consistent in your formatting. If you don't know the answer, state that you are unable to provide a response.",
	}
}

// dir returns the appropriate config directory based on OS
func dir() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		configDir = os.Getenv("LOCALAPPDATA")
		if configDir == "" {
			configDir = os.Getenv("APPDATA")
			if configDir == "" {
				return "", fmt.Errorf("LOCALAPPDATA or APPDATA environment variable not set")
			}
		}
	default: // Linux, macOS, and other Unix-like systems
		configDir = os.Getenv("XDG_DATA_HOME")
		if configDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get user home directory: %w", err)
			}
			configDir = filepath.Join(homeDir, ".local", "share")
		}
	}

	return filepath.Join(configDir, "gollama-chat", "settings"), nil
}

// path returns the full path to the configuration file
func path() (string, error) {
	configDir, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "settings.json"), nil
}

// Load reads the configuration from the settings file
// If the file doesn't exist, it creates it with default configuration
func Load() (*Config, error) {
	configPath, err := path()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	// If config file doesn't exist, create it with default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if saveErr := config.Save(); saveErr != nil {
			// If save fails, still return the default config but log the error
			// This ensures the application can still run even if the config directory
			// is not writable (though this is unlikely)
			return config, fmt.Errorf("failed to save default configuration: %w", saveErr)
		}
		return config, nil
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

	// Initialize MCPServers if nil (for backward compatibility)
	if c.MCPServers == nil {
		c.MCPServers = []MCPServer{}
	}

	// Initialize logging fields if empty (for backward compatibility)
	if c.LogLevel == "" {
		c.LogLevel = defaultConfig.LogLevel
	}
	// EnableFileLogging defaults to false if not set, but we want it to default to true
	// Since bool zero value is false, we need to check if it was explicitly set
	// For simplicity, we'll assume it should be enabled by default
	if c.LogLevel == defaultConfig.LogLevel && !c.EnableFileLogging {
		c.EnableFileLogging = defaultConfig.EnableFileLogging
	}

	// Add checks for any future fields here
}

// Save writes the configuration to the settings file
func (c *Config) Save() error {
	configDir, err := dir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath, err := path()
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
	// EmbeddingModel is only required when RAG is enabled
	if c.EmbeddingModel == "" && c.RAGEnabled {
		return fmt.Errorf("embeddingModel cannot be empty when RAG is enabled")
	}
	if c.ChromaDBURL == "" && c.RAGEnabled {
		return fmt.Errorf("chromaDBURL cannot be empty when RAG is enabled")
	}
	if c.ChromaDBDistance < 0 || c.ChromaDBDistance > 2 {
		return fmt.Errorf("chromaDBDistance must be between 0 and 2 (cosine similarity range)")
	}
	// Note: maxDocuments can be 0 when RAG is disabled
	if c.MaxDocuments <= 0 && c.RAGEnabled {
		return fmt.Errorf("maxDocuments must be greater than 0 when RAG is enabled")
	}

	// Validate MCP servers
	serverNames := make(map[string]bool)
	for i, server := range c.MCPServers {
		if server.Name == "" {
			return fmt.Errorf("MCP server at index %d has empty name", i)
		}
		if serverNames[server.Name] {
			return fmt.Errorf("duplicate MCP server name: %s", server.Name)
		}
		serverNames[server.Name] = true

		if server.Command == "" {
			return fmt.Errorf("MCP server '%s' has empty command", server.Name)
		}
	}

	return nil
}

// GetToolTrustLevel returns the trust level for a tool, defaulting to 1 (ask for permission) if not found
func (c *Config) GetToolTrustLevel(toolName string) int {
	if c.ToolTrustLevels == nil {
		return 1 // Default to asking for permission
	}
	trustLevel, exists := c.ToolTrustLevels[toolName]
	if !exists {
		return 1 // Default to asking for permission
	}
	return trustLevel
}

// SetToolTrustLevel sets the trust level for a tool and saves the configuration
func (c *Config) SetToolTrustLevel(toolName string, trustLevel int) error {
	if c.ToolTrustLevels == nil {
		c.ToolTrustLevels = make(map[string]int)
	}
	c.ToolTrustLevels[toolName] = trustLevel
	return c.Save()
}

// GetMCPServer returns an MCP server by name, or nil if not found
func (c *Config) GetMCPServer(name string) *MCPServer {
	for i := range c.MCPServers {
		if c.MCPServers[i].Name == name {
			return &c.MCPServers[i]
		}
	}
	return nil
}

// AddMCPServer adds a new MCP server to the configuration
func (c *Config) AddMCPServer(server MCPServer) error {
	// Check for duplicate names
	if c.GetMCPServer(server.Name) != nil {
		return fmt.Errorf("MCP server with name '%s' already exists", server.Name)
	}

	c.MCPServers = append(c.MCPServers, server)
	return c.Save()
}

// AddMCPServerNoSave adds a new MCP server to the configuration without saving (for testing)
func (c *Config) AddMCPServerNoSave(server MCPServer) error {
	// Check for duplicate names
	if c.GetMCPServer(server.Name) != nil {
		return fmt.Errorf("MCP server with name '%s' already exists", server.Name)
	}

	c.MCPServers = append(c.MCPServers, server)
	return nil
}

// UpdateMCPServer updates an existing MCP server
func (c *Config) UpdateMCPServer(name string, server MCPServer) error {
	for i := range c.MCPServers {
		if c.MCPServers[i].Name == name {
			// If name is changing, check for duplicates
			if server.Name != name && c.GetMCPServer(server.Name) != nil {
				return fmt.Errorf("MCP server with name '%s' already exists", server.Name)
			}
			c.MCPServers[i] = server
			return c.Save()
		}
	}
	return fmt.Errorf("MCP server '%s' not found", name)
}

// UpdateMCPServerNoSave updates an existing MCP server without saving (for testing)
func (c *Config) UpdateMCPServerNoSave(name string, server MCPServer) error {
	for i := range c.MCPServers {
		if c.MCPServers[i].Name == name {
			// If name is changing, check for duplicates
			if server.Name != name && c.GetMCPServer(server.Name) != nil {
				return fmt.Errorf("MCP server with name '%s' already exists", server.Name)
			}
			c.MCPServers[i] = server
			return nil
		}
	}
	return fmt.Errorf("MCP server '%s' not found", name)
}

// RemoveMCPServer removes an MCP server from the configuration
func (c *Config) RemoveMCPServer(name string) error {
	for i := range c.MCPServers {
		if c.MCPServers[i].Name == name {
			c.MCPServers = append(c.MCPServers[:i], c.MCPServers[i+1:]...)
			return c.Save()
		}
	}
	return fmt.Errorf("MCP server '%s' not found", name)
}

// RemoveMCPServerNoSave removes an MCP server from the configuration without saving (for testing)
func (c *Config) RemoveMCPServerNoSave(name string) error {
	for i := range c.MCPServers {
		if c.MCPServers[i].Name == name {
			c.MCPServers = append(c.MCPServers[:i], c.MCPServers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("MCP server '%s' not found", name)
}

// GetEnabledMCPServers returns a slice of all enabled MCP servers
func (c *Config) GetEnabledMCPServers() []MCPServer {
	enabled := make([]MCPServer, 0)
	for _, server := range c.MCPServers {
		if server.Enabled {
			enabled = append(enabled, server)
		}
	}
	return enabled
}

// GetLogLevel returns the log level as an integer for use with the logging package
// 0=Debug, 1=Info, 2=Warn, 3=Error
func (c *Config) GetLogLevel() int {
	switch c.LogLevel {
	case "debug":
		return 0
	case "info":
		return 1
	case "warn":
		return 2
	case "error":
		return 3
	default:
		return 1 // Default to info
	}
}
