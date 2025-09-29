package tooling

import (
	"os"
	"path/filepath"
	"testing"
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
