package tooling

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ollama/ollama/api"
)

// Global registry instance
var DefaultRegistry *ToolRegistry

// init initializes the default registry and registers built-in tools
func init() {
	DefaultRegistry = NewToolRegistry()

	// Register built-in tools
	DefaultRegistry.Register(&FileSystemTool{})
}

// ToolRegistry holds all registered tools
type ToolRegistry struct {
	tools map[string]BuiltinTool
}

// BuiltinTool represents a built-in tool that can be registered
type BuiltinTool interface {
	Name() string
	Description() string
	GetAPITool() *api.Tool
	Execute(args map[string]interface{}) (interface{}, error)
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]BuiltinTool),
	}
}

// Register adds a tool to the registry
func (tr *ToolRegistry) Register(tool BuiltinTool) {
	tr.tools[tool.Name()] = tool
}

// GetTool retrieves a tool by name
func (tr *ToolRegistry) GetTool(name string) (BuiltinTool, bool) {
	tool, exists := tr.tools[name]
	return tool, exists
}

// GetAllTools returns all registered tools
func (tr *ToolRegistry) GetAllTools() map[string]BuiltinTool {
	return tr.tools
}

// FileSystemTool provides filesystem operations
type FileSystemTool struct{}

// FileInfo represents file/directory information
type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"is_dir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Mode     string    `json:"mode"`
}

// ListDirectoryArgs represents arguments for listing a directory
type ListDirectoryArgs struct {
	Path string `json:"path"`
}

// ReadFileArgs represents arguments for reading a file
type ReadFileArgs struct {
	Path     string `json:"path"`
	MaxBytes int    `json:"max_bytes,omitempty"` // Optional limit on file size
	Encoding string `json:"encoding,omitempty"`  // Optional encoding (default: utf-8)
}

// Name returns the tool name
func (fst *FileSystemTool) Name() string {
	return "filesystem_read"
}

// Description returns the tool description
func (fst *FileSystemTool) Description() string {
	return "Read local filesystem - get working directory, list directories and read file contents"
}

// GetAPITool returns the Ollama API tool definition
func (fst *FileSystemTool) GetAPITool() *api.Tool {
	return &api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "filesystem_read",
			Description: "Read local filesystem - get working directory, list directories and read file contents",
			Parameters: api.ToolFunctionParameters{
				Type: "object",
				Properties: map[string]api.ToolProperty{
					"action": {
						Type:        api.PropertyType{"string"},
						Description: "Action to perform: 'get_working_directory', 'list_directory' or 'read_file'",
						Enum:        []any{"get_working_directory", "list_directory", "read_file"},
					},
					"path": {
						Type:        api.PropertyType{"string"},
						Description: "File or directory path (not required for get_working_directory)",
					},
					"max_bytes": {
						Type:        api.PropertyType{"integer"},
						Description: "Maximum bytes to read for files (optional, default: 10MB)",
					},
				},
				Required: []string{"action"},
			},
		},
	}
}

// Execute performs the filesystem operation
func (fst *FileSystemTool) Execute(args map[string]interface{}) (interface{}, error) {
	action, ok := args["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action parameter required and must be a string")
	}

	switch action {
	case "get_working_directory":
		return fst.getWorkingDirectory()
	case "list_directory", "read_file":
		// These actions require a path parameter
		path, ok := args["path"].(string)
		if !ok {
			return nil, fmt.Errorf("path parameter required for action '%s' and must be a string", action)
		}

		// Clean and validate the path
		cleanPath := filepath.Clean(path)

		switch action {
		case "list_directory":
			return fst.listDirectory(cleanPath)
		case "read_file":
			maxBytes := 10 * 1024 * 1024 // 10MB default
			if mb, ok := args["max_bytes"].(float64); ok {
				maxBytes = int(mb)
			}
			return fst.readFile(cleanPath, maxBytes)
		}
	}

	return nil, fmt.Errorf("unknown action: %s. Valid actions are: get_working_directory, list_directory, read_file", action)
}

// listDirectory lists files and directories in the given path
func (fst *FileSystemTool) listDirectory(path string) (interface{}, error) {
	// Check if path exists and is accessible
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access path %s: %v", path, err)
	}

	if !stat.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", path)
	}

	// Read directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory %s: %v", path, err)
	}

	var fileInfos []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			// Skip entries that can't be accessed
			continue
		}

		fileInfo := FileInfo{
			Name:     entry.Name(),
			Path:     filepath.Join(path, entry.Name()),
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
			Mode:     info.Mode().String(),
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	return map[string]interface{}{
		"path":     path,
		"entries":  fileInfos,
		"count":    len(fileInfos),
		"readable": true,
	}, nil
}

// readFile reads the contents of a file with size limits
func (fst *FileSystemTool) readFile(path string, maxBytes int) (interface{}, error) {
	// Check if path exists and is accessible
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access file %s: %v", path, err)
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("path %s is a directory, not a file", path)
	}

	// Check file size
	if stat.Size() > int64(maxBytes) {
		return nil, fmt.Errorf("file %s is too large (%d bytes, max: %d bytes)", path, stat.Size(), maxBytes)
	}

	// Open and read the file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %s: %v", path, err)
	}
	defer file.Close()

	// Read up to maxBytes
	content := make([]byte, maxBytes)
	n, err := io.ReadFull(file, content)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, fmt.Errorf("error reading file %s: %v", path, err)
	}

	content = content[:n] // Trim to actual size

	// Check if content is valid UTF-8
	isText := utf8.Valid(content)
	var contentStr string
	var isBinary bool

	if isText {
		contentStr = string(content)
		isBinary = false
	} else {
		// For binary files, provide basic info but don't include raw content
		contentStr = fmt.Sprintf("<binary file - %d bytes>", len(content))
		isBinary = true
	}

	// Detect if file looks like a text file based on content
	if isText && len(content) > 0 {
		// Check for common binary indicators in the first few bytes
		hasNullBytes := strings.Contains(string(content[:min(512, len(content))]), "\x00")
		if hasNullBytes {
			isBinary = true
			contentStr = fmt.Sprintf("<binary file with null bytes - %d bytes>", len(content))
		}
	}

	return map[string]interface{}{
		"path":       path,
		"size":       stat.Size(),
		"modified":   stat.ModTime(),
		"mode":       stat.Mode().String(),
		"content":    contentStr,
		"is_binary":  isBinary,
		"is_text":    !isBinary,
		"bytes_read": len(content),
	}, nil
}

// getWorkingDirectory returns the current working directory
func (fst *FileSystemTool) getWorkingDirectory() (interface{}, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot get working directory: %v", err)
	}

	// Get directory info
	stat, err := os.Stat(wd)
	if err != nil {
		return nil, fmt.Errorf("cannot access working directory %s: %v", wd, err)
	}

	// Get absolute path
	absPath, err := filepath.Abs(wd)
	if err != nil {
		absPath = wd // Fallback to original path
	}

	return map[string]interface{}{
		"path":     wd,
		"abs_path": absPath,
		"mode":     stat.Mode().String(),
		"modified": stat.ModTime(),
		"exists":   true,
		"is_dir":   stat.IsDir(),
	}, nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
