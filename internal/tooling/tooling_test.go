package tooling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ollama/ollama/api"
)

func TestFileSystemTool_GetWorkingDirectory(t *testing.T) {
	fst := &FileSystemTool{}

	args := map[string]any{
		"action": "get_working_directory",
	}

	result, err := fst.Execute(args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
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

	args := map[string]any{
		"action": "list_directory",
		"path":   wd,
	}

	result, err := fst.Execute(args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
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

	args := map[string]any{
		"action": "read_file",
		"path":   tempFile,
	}

	result, err := fst.Execute(args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
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

	args := map[string]any{
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

func (m *MockBuiltinTool) Execute(args map[string]any) (any, error) {
	return map[string]any{
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
	args := map[string]any{
		"param1": "value1",
		"param2": 42,
	}

	result, err := registry.ExecuteTool("executable_tool", args)
	if err != nil {
		t.Errorf("Tool execution should not fail: %v", err)
	}

	resultMap, ok := result.(map[string]any)
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
	_, err := DefaultRegistry.ExecuteTool("filesystem_read", map[string]any{
		"operation": "get_working_directory",
	})

	// We don't check for success here since it depends on filesystem,
	// but it should not panic or return a "tool not found" error
	if err != nil && err.Error() == "tool filesystem_read not found" {
		t.Error("filesystem_read tool should be found in registry")
	}
}

// TestExecuteBashTool_BasicExecution tests basic command execution
func TestExecuteBashTool_BasicExecution(t *testing.T) {
	ebt := &ExecuteBashTool{}

	tests := []struct {
		name        string
		command     string
		expectError bool
		checkFields []string
	}{
		{
			name:        "echo command",
			command:     "echo 'Hello World'",
			expectError: false,
			checkFields: []string{"stdout", "stderr", "exit_code", "success", "duration_ms"},
		},
		{
			name:        "pwd command",
			command:     "pwd",
			expectError: false,
			checkFields: []string{"stdout", "stderr", "exit_code", "success", "duration_ms"},
		},
		{
			name:        "date command",
			command:     "date",
			expectError: false,
			checkFields: []string{"stdout", "stderr", "exit_code", "success", "duration_ms"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{
				"command": tt.command,
			}

			result, err := ebt.Execute(args)
			if (err != nil) != tt.expectError {
				t.Errorf("Execute() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				resultMap, ok := result.(map[string]any)
				if !ok {
					t.Fatalf("Expected result to be map[string]interface{}, got %T", result)
				}

				// Check all required fields are present
				for _, field := range tt.checkFields {
					if _, ok := resultMap[field]; !ok {
						t.Errorf("Result missing '%s' field", field)
					}
				}

				// Check command field matches input
				if resultMap["command"] != tt.command {
					t.Errorf("Expected command '%s', got '%s'", tt.command, resultMap["command"])
				}

				// For successful commands, exit_code should be 0
				if exitCode, ok := resultMap["exit_code"].(int); ok && exitCode == 0 {
					if success, ok := resultMap["success"].(bool); !ok || !success {
						t.Error("Expected success to be true for exit code 0")
					}
				}
			}
		})
	}
}

// TestExecuteBashTool_WorkingDirectory tests command execution with working directory
func TestExecuteBashTool_WorkingDirectory(t *testing.T) {
	ebt := &ExecuteBashTool{}

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	args := map[string]any{
		"command":     "pwd",
		"working_dir": tempDir,
	}

	result, err := ebt.Execute(args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}, got %T", result)
	}

	// Check that working_dir field is set correctly
	if resultMap["working_dir"] != tempDir {
		t.Errorf("Expected working_dir '%s', got '%s'", tempDir, resultMap["working_dir"])
	}

	// Check that stdout contains the temp directory path
	stdout, ok := resultMap["stdout"].(string)
	if !ok {
		t.Error("stdout should be a string")
	}

	// The output should contain the temp directory path
	if !filepath.IsAbs(stdout) && !filepath.IsAbs(tempDir) {
		// Convert both to absolute paths for comparison
		absTemp, _ := filepath.Abs(tempDir)
		absStdout, _ := filepath.Abs(strings.TrimSpace(stdout))
		if absStdout != absTemp {
			t.Errorf("pwd output '%s' doesn't match expected working directory '%s'", absStdout, absTemp)
		}
	}
}

// TestExecuteBashTool_ErrorHandling tests various error conditions
func TestExecuteBashTool_ErrorHandling(t *testing.T) {
	ebt := &ExecuteBashTool{}

	tests := []struct {
		name        string
		args        map[string]any
		expectError bool
		errorText   string
	}{
		{
			name:        "missing command",
			args:        map[string]any{},
			expectError: true,
			errorText:   "command parameter required",
		},
		{
			name: "empty command",
			args: map[string]any{
				"command": "",
			},
			expectError: true,
			errorText:   "command cannot be empty",
		},
		{
			name: "non-string command",
			args: map[string]any{
				"command": 123,
			},
			expectError: true,
			errorText:   "command parameter required and must be a string",
		},
		{
			name: "invalid working directory",
			args: map[string]any{
				"command":     "echo test",
				"working_dir": "/nonexistent/directory/path",
			},
			expectError: true,
			errorText:   "working directory",
		},
		{
			name: "working directory is file",
			args: map[string]any{
				"command":     "echo test",
				"working_dir": "/etc/passwd", // This is a file, not a directory
			},
			expectError: true,
			errorText:   "is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ebt.Execute(tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("Execute() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if tt.expectError && err != nil {
				if !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorText, err.Error())
				}
			}

			if !tt.expectError && result == nil {
				t.Error("Expected result to not be nil for successful execution")
			}
		})
	}
}

// TestExecuteBashTool_CommandFailure tests commands that fail with non-zero exit codes
func TestExecuteBashTool_CommandFailure(t *testing.T) {
	ebt := &ExecuteBashTool{}

	args := map[string]any{
		"command": "exit 1", // Command that exits with code 1
	}

	result, err := ebt.Execute(args)
	if err != nil {
		t.Fatalf("Execute should not return error for command with non-zero exit code: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}, got %T", result)
	}

	// Check exit code is 1
	if exitCode, ok := resultMap["exit_code"].(int); !ok || exitCode != 1 {
		t.Errorf("Expected exit_code to be 1, got %v", resultMap["exit_code"])
	}

	// Check success is false
	if success, ok := resultMap["success"].(bool); !ok || success {
		t.Error("Expected success to be false for non-zero exit code")
	}
}

// TestExecuteBashTool_Timeout tests command timeout functionality
func TestExecuteBashTool_Timeout(t *testing.T) {
	ebt := &ExecuteBashTool{}

	// Use a command that will definitely exist and sleep
	args := map[string]any{
		"command": "bash -c 'sleep 3'", // Command that takes 3 seconds
		"timeout": 1,                   // 1 second timeout
	}

	result, err := ebt.Execute(args)
	if err == nil {
		t.Error("Expected timeout error for long-running command")
		if result != nil {
			t.Logf("Unexpected result: %+v", result)
		}
		return
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Expected timeout error message, got: %v", err)
	}

	// Result should be nil on timeout
	if result != nil {
		t.Error("Expected result to be nil on timeout")
	}
}

// TestExecuteBashTool_TimeoutBoundaries tests timeout parameter validation
func TestExecuteBashTool_TimeoutBoundaries(t *testing.T) {
	ebt := &ExecuteBashTool{}

	tests := []struct {
		name            string
		timeout         float64
		expectedTimeout int
	}{
		{
			name:            "negative timeout defaults to 30",
			timeout:         -5,
			expectedTimeout: 30,
		},
		{
			name:            "zero timeout defaults to 30",
			timeout:         0,
			expectedTimeout: 30,
		},
		{
			name:            "timeout over 300 capped at 300",
			timeout:         500,
			expectedTimeout: 300,
		},
		{
			name:            "valid timeout preserved",
			timeout:         60,
			expectedTimeout: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{
				"command": "echo test",
				"timeout": tt.timeout,
			}

			result, err := ebt.Execute(args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("Expected result to be map[string]interface{}, got %T", result)
			}

			// Check that timeout field reflects the expected value
			if timeout, ok := resultMap["timeout"].(int); !ok || timeout != tt.expectedTimeout {
				t.Errorf("Expected timeout %d, got %v", tt.expectedTimeout, resultMap["timeout"])
			}
		})
	}
}

// TestExecuteBashTool_Registration tests that the execute_bash tool is properly registered
func TestExecuteBashTool_Registration(t *testing.T) {
	// Test that the execute_bash tool is registered in the default registry
	tool, exists := DefaultRegistry.GetTool("execute_bash")
	if !exists {
		t.Error("ExecuteBash tool not found in default registry")
	}

	if tool.Name() != "execute_bash" {
		t.Errorf("Expected tool name 'execute_bash', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Tool description is empty")
	}

	apiTool := tool.GetAPITool()
	if apiTool == nil {
		t.Error("API tool is nil")
	}

	if apiTool.Function.Name != "execute_bash" {
		t.Errorf("Expected API tool name 'execute_bash', got '%s'", apiTool.Function.Name)
	}

	// Check required parameters
	params := apiTool.Function.Parameters
	if params.Type != "object" {
		t.Error("Expected parameters type to be 'object'")
	}

	// Check that command parameter exists and is required
	if _, ok := params.Properties["command"]; !ok {
		t.Error("Missing 'command' parameter in API tool definition")
	}

	foundCommand := false
	for _, required := range params.Required {
		if required == "command" {
			foundCommand = true
			break
		}
	}
	if !foundCommand {
		t.Error("'command' should be in required parameters")
	}
}

// TestExecuteBashTool_Interface verifies ExecuteBashTool implements BuiltinTool interface
func TestExecuteBashTool_Interface(t *testing.T) {
	var _ BuiltinTool = &ExecuteBashTool{}
}
