package tooling

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ollama/ollama/api"
)

func TestFileSystemTool_GetWorkingDirectory(t *testing.T) {
	fst := &FileSystemTool{}

	args := map[string]interface{}{
		"action": "get_working_directory",
	}

	result, err := fst.Execute(args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}, got %T", result)
	}

	// Check required fields
	if _, ok := resultMap["path"]; !ok {
		t.Error("Result missing 'path' field")
	}
	if _, ok := resultMap["abs_path"]; !ok {
		t.Error("Result missing 'abs_path' field")
	}
	if _, ok := resultMap["exists"]; !ok {
		t.Error("Result missing 'exists' field")
	}

	// Verify the path matches os.Getwd()
	expectedWd, _ := os.Getwd()
	if resultMap["path"] != expectedWd {
		t.Errorf("Expected path '%s', got '%s'", expectedWd, resultMap["path"])
	}

	// Verify it exists and is a directory
	if resultMap["exists"] != true {
		t.Error("Expected exists to be true")
	}
	if resultMap["is_dir"] != true {
		t.Error("Expected is_dir to be true")
	}
}

func TestFileSystemTool_ListDirectory(t *testing.T) {
	fst := &FileSystemTool{}

	// Test with current directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	args := map[string]interface{}{
		"action": "list_directory",
		"path":   wd,
	}

	result, err := fst.Execute(args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}, got %T", result)
	}

	// Check required fields
	if _, ok := resultMap["path"]; !ok {
		t.Error("Result missing 'path' field")
	}
	if _, ok := resultMap["entries"]; !ok {
		t.Error("Result missing 'entries' field")
	}
	if _, ok := resultMap["count"]; !ok {
		t.Error("Result missing 'count' field")
	}
}

func TestFileSystemTool_ReadFile(t *testing.T) {
	fst := &FileSystemTool{}

	// Create a temporary test file
	tempFile := filepath.Join(os.TempDir(), "test_file.txt")
	content := "Hello, World!\nThis is a test file."
	err := os.WriteFile(tempFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tempFile)

	args := map[string]interface{}{
		"action": "read_file",
		"path":   tempFile,
	}

	result, err := fst.Execute(args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}, got %T", result)
	}

	// Check required fields
	if _, ok := resultMap["path"]; !ok {
		t.Error("Result missing 'path' field")
	}
	if _, ok := resultMap["content"]; !ok {
		t.Error("Result missing 'content' field")
	}
	if _, ok := resultMap["is_binary"]; !ok {
		t.Error("Result missing 'is_binary' field")
	}

	// Check content
	if resultMap["content"] != content {
		t.Errorf("Expected content '%s', got '%s'", content, resultMap["content"])
	}

	// Check it's detected as text
	if resultMap["is_binary"] != false {
		t.Error("Expected is_binary to be false for text file")
	}
}

func TestFileSystemTool_InvalidAction(t *testing.T) {
	fst := &FileSystemTool{}

	args := map[string]interface{}{
		"action": "invalid_action",
		"path":   "/tmp",
	}

	_, err := fst.Execute(args)
	if err == nil {
		t.Error("Expected error for invalid action")
	}
}

func TestFileSystemTool_Registration(t *testing.T) {
	// Test that the filesystem tool is registered in the default registry
	tool, exists := DefaultRegistry.GetTool("filesystem_read")
	if !exists {
		t.Error("Filesystem tool not found in default registry")
	}

	if tool.Name() != "filesystem_read" {
		t.Errorf("Expected tool name 'filesystem_read', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Tool description is empty")
	}

	apiTool := tool.GetAPITool()
	if apiTool == nil {
		t.Error("API tool is nil")
	}

	if apiTool.Function.Name != "filesystem_read" {
		t.Errorf("Expected API tool name 'filesystem_read', got '%s'", apiTool.Function.Name)
	}
}

// MockBuiltinTool is a test implementation of BuiltinTool
type MockBuiltinTool struct {
	name        string
	description string
}

func (m *MockBuiltinTool) Name() string {
	return m.name
}

func (m *MockBuiltinTool) Description() string {
	return m.description
}

func (m *MockBuiltinTool) GetAPITool() *api.Tool {
	return &api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        m.name,
			Description: m.description,
			Parameters: api.ToolFunctionParameters{
				Type:       "object",
				Properties: make(map[string]api.ToolProperty),
				Required:   []string{},
			},
		},
	}
}

func (m *MockBuiltinTool) Execute(args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"tool":   m.name,
		"result": "mock execution result",
		"args":   args,
	}, nil
}

func TestToolRegistry_Register(t *testing.T) {
	registry := NewToolRegistry()

	tool := &MockBuiltinTool{
		name:        "test_tool",
		description: "A test tool for unit testing",
	}

	// Test registration
	registry.Register(tool)

	// Verify tool was registered in builtin tools
	builtinTool, exists := registry.GetTool("test_tool")
	if !exists {
		t.Error("Tool should exist in builtin tools after registration")
	}

	if builtinTool.Name() != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", builtinTool.Name())
	}

	// Verify unified tool was created
	unifiedTool, exists := registry.GetUnifiedTool("test_tool")
	if !exists {
		t.Error("Tool should exist in unified tools after registration")
	}

	if unifiedTool.Name != "test_tool" {
		t.Errorf("Expected unified tool name 'test_tool', got '%s'", unifiedTool.Name)
	}

	if unifiedTool.Source != "builtin" {
		t.Errorf("Expected source 'builtin', got '%s'", unifiedTool.Source)
	}

	if !unifiedTool.Available {
		t.Error("Builtin tool should be available")
	}
}

func TestToolRegistry_GetAllTools(t *testing.T) {
	registry := NewToolRegistry()

	// Register multiple tools
	tools := []*MockBuiltinTool{
		{name: "tool1", description: "First tool"},
		{name: "tool2", description: "Second tool"},
		{name: "tool3", description: "Third tool"},
	}

	for _, tool := range tools {
		registry.Register(tool)
	}

	// Get all tools
	allTools := registry.GetAllTools()

	if len(allTools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(allTools))
	}

	for _, tool := range tools {
		if _, exists := allTools[tool.name]; !exists {
			t.Errorf("Tool '%s' should exist in all tools", tool.name)
		}
	}
}

func TestToolRegistry_ExecuteTool(t *testing.T) {
	registry := NewToolRegistry()

	tool := &MockBuiltinTool{
		name:        "executable_tool",
		description: "A tool for testing execution",
	}

	registry.Register(tool)

	// Test successful execution
	args := map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	}

	result, err := registry.ExecuteTool("executable_tool", args)
	if err != nil {
		t.Errorf("Tool execution should not fail: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Error("Result should be a map")
	}

	if resultMap["tool"] != "executable_tool" {
		t.Errorf("Expected tool name in result, got %v", resultMap["tool"])
	}

	// Test execution of non-existent tool
	_, err = registry.ExecuteTool("non_existent_tool", args)
	if err == nil {
		t.Error("Execution of non-existent tool should fail")
	}

	if err.Error() != "tool non_existent_tool not found" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestDefaultRegistry_FileSystemTool(t *testing.T) {
	// Test that the default registry has the filesystem tool registered
	allTools := DefaultRegistry.GetAllTools()

	if len(allTools) == 0 {
		t.Error("Default registry should have at least one tool")
	}

	if _, exists := allTools["filesystem_read"]; !exists {
		t.Error("Default registry should have filesystem_read tool")
	}

	// Test that the tool is available in unified tools
	unifiedTools := DefaultRegistry.GetAllUnifiedTools()
	if _, exists := unifiedTools["filesystem_read"]; !exists {
		t.Error("Default registry should have filesystem_read in unified tools")
	}

	// Test the tool can be executed (basic validation)
	_, err := DefaultRegistry.ExecuteTool("filesystem_read", map[string]interface{}{
		"operation": "get_working_directory",
	})

	// We don't check for success here since it depends on filesystem,
	// but it should not panic or return a "tool not found" error
	if err != nil && err.Error() == "tool filesystem_read not found" {
		t.Error("filesystem_read tool should be found in registry")
	}
}
