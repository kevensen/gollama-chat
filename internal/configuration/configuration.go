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
	// DefaultSystemPrompt is deprecated - system prompt is now stored in SYSTEM_PROMPT.md
	DefaultSystemPrompt string          `json:"defaultSystemPrompt,omitempty"` // Keep for migration
	ToolTrustLevels     map[string]int  `json:"toolTrustLevels"`   // Maps tool name to trust level: 0=None(block), 1=Ask(prompt), 2=Session(allow)
	MCPServers          []MCPServer     `json:"mcpServers"`        // MCP server configurations
	LogLevel            string          `json:"logLevel"`          // Log level: debug, info, warn, error
	EnableFileLogging   bool            `json:"enableFileLogging"` // Whether to log to file
	AgentsFileEnabled   bool            `json:"agentsFileEnabled"` // Whether to automatically detect and use AGENTS.md files
	
	// systemPrompt is the cached system prompt content from SYSTEM_PROMPT.md
	// This field is not serialized to JSON
	systemPrompt string
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	// Create default tool trust levels with hardcoded values for built-in tools
	defaultToolTrustLevels := make(map[string]int)
	defaultToolTrustLevels["execute_bash"] = 1 // Hardcoded: ask for permission

	return &Config{
		ChatModel:           "llama3.3:latest",
		EmbeddingModel:      "nomic-embed-text:latest",
		RAGEnabled:          false,
		OllamaURL:           "http://localhost:11434",
		ChromaDBURL:         "http://localhost:8000",
		ChromaDBDistance:    1.0, // Updated for cosine similarity (0-2 range)
		MaxDocuments:        5,
		SelectedCollections: make(map[string]bool),
		ToolTrustLevels:     defaultToolTrustLevels,
		MCPServers:          []MCPServer{},
		LogLevel:            "info",
		EnableFileLogging:   true,
		AgentsFileEnabled:   true, // Enable AGENTS.md detection by default
		// DefaultSystemPrompt is left empty - system prompt is now loaded from SYSTEM_PROMPT.md
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

// systemPromptPath returns the full path to the system prompt markdown file
func systemPromptPath() (string, error) {
	configDir, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "SYSTEM_PROMPT.md"), nil
}

// getDefaultSystemPrompt returns the default system prompt content
func getDefaultSystemPrompt() string {
	return `You are a helpful AI assistant with access to system tools. Provide concise, well-formatted responses. If you don't know something, state "I don't know" rather than guessing.

## Tool Access & Permissions
You have access to tools that require user permission. When requesting tool use, explain what tool you need and why. The permission system uses:
- **y**: Allow this specific use
- **n**: Deny this request  
- **t**: Trust for entire session

Available tools include execute_bash, filesystem_read, MCP tools (server.toolname format), and RAG (no permission needed).

## Project Context
If an AGENTS.md file is detected, you must evaluate and follow all instructions in that file. Project-specific guidance takes priority over default behavior.

Your goal is to provide accurate, helpful assistance while respecting permissions and following project-specific guidance.`
}

// LoadSystemPrompt reads the system prompt from SYSTEM_PROMPT.md
// If the file doesn't exist, it creates it with default content
func LoadSystemPrompt() (string, error) {
	promptPath, err := systemPromptPath()
	if err != nil {
		return "", fmt.Errorf("failed to get system prompt path: %w", err)
	}

	// If system prompt file doesn't exist, create it with default content
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		configDir, err := dir()
		if err != nil {
			return "", fmt.Errorf("failed to get config directory: %w", err)
		}

		// Create config directory if it doesn't exist
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create config directory: %w", err)
		}

		defaultPrompt := getDefaultSystemPrompt()
		if err := os.WriteFile(promptPath, []byte(defaultPrompt), 0644); err != nil {
			return "", fmt.Errorf("failed to create default system prompt file: %w", err)
		}
		return defaultPrompt, nil
	}

	data, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read system prompt file: %w", err)
	}

	return string(data), nil
}

// SaveSystemPrompt writes the system prompt to SYSTEM_PROMPT.md
func SaveSystemPrompt(prompt string) error {
	configDir, err := dir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	promptPath, err := systemPromptPath()
	if err != nil {
		return fmt.Errorf("failed to get system prompt path: %w", err)
	}

	if err := os.WriteFile(promptPath, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("failed to write system prompt file: %w", err)
	}

	return nil
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
		// Ensure system prompt file exists with default content
		_, err := LoadSystemPrompt()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize system prompt: %w", err)
		}
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

	// Handle migration from old DefaultSystemPrompt to new file-based system
	if err := migrateSystemPrompt(&config); err != nil {
		return nil, fmt.Errorf("failed to migrate system prompt: %w", err)
	}

	return &config, nil
}

// migrateSystemPrompt handles migration from old DefaultSystemPrompt field to SYSTEM_PROMPT.md file
func migrateSystemPrompt(c *Config) error {
	// Check if there's a system prompt file already
	promptPath, err := systemPromptPath()
	if err != nil {
		return fmt.Errorf("failed to get system prompt path: %w", err)
	}

	_, fileExists := os.Stat(promptPath)
	
	// If we have a DefaultSystemPrompt in the config and no file exists, migrate it
	if c.DefaultSystemPrompt != "" && os.IsNotExist(fileExists) {
		if err := SaveSystemPrompt(c.DefaultSystemPrompt); err != nil {
			return fmt.Errorf("failed to migrate system prompt to file: %w", err)
		}
		// Clear the JSON field after successful migration and save config
		c.DefaultSystemPrompt = ""
		if err := c.Save(); err != nil {
			return fmt.Errorf("failed to save config after migration: %w", err)
		}
	} else if os.IsNotExist(fileExists) {
		// No file exists and no DefaultSystemPrompt, ensure default file is created
		_, err := LoadSystemPrompt()
		if err != nil {
			return fmt.Errorf("failed to create default system prompt: %w", err)
		}
	}

	return nil
}

// applyDefaultsIfMissing sets default values for any config fields that might be missing
// This ensures backward compatibility when new fields are added
func applyDefaultsIfMissing(c *Config) {
	defaultConfig := DefaultConfig()

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

// GetSystemPrompt returns the system prompt, loading it from file if not cached
func (c *Config) GetSystemPrompt() (string, error) {
	if c.systemPrompt == "" {
		prompt, err := LoadSystemPrompt()
		if err != nil {
			return "", err
		}
		c.systemPrompt = prompt
	}
	return c.systemPrompt, nil
}

// SetSystemPrompt sets the system prompt and saves it to file
func (c *Config) SetSystemPrompt(prompt string) error {
	if err := SaveSystemPrompt(prompt); err != nil {
		return err
	}
	c.systemPrompt = prompt
	return nil
}

// GetDefaultSystemPrompt returns the system prompt for backward compatibility
// This is deprecated - use GetSystemPrompt() instead
func (c *Config) GetDefaultSystemPrompt() (string, error) {
	return c.GetSystemPrompt()
}
