package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDetector(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"detector enabled", true},
		{"detector disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewDetector(tt.enabled)
			if detector == nil {
				t.Fatal("NewDetector returned nil")
			}
			if detector.IsEnabled() != tt.enabled {
				t.Errorf("IsEnabled() = %v, want %v", detector.IsEnabled(), tt.enabled)
			}
		})
	}
}

func TestDetector_SetEnabled(t *testing.T) {
	detector := NewDetector(false)

	// Test enabling
	detector.SetEnabled(true)
	if !detector.IsEnabled() {
		t.Error("Expected detector to be enabled")
	}

	// Test disabling
	detector.SetEnabled(false)
	if detector.IsEnabled() {
		t.Error("Expected detector to be disabled")
	}
}

func TestDetector_DetectInDirectory(t *testing.T) {
	tests := []struct {
		name            string
		enabled         bool
		setupFunc       func(dir string) error
		expectFile      bool
		expectError     bool
		expectedPath    string
		expectedContent string
	}{
		{
			name:    "detector disabled",
			enabled: false,
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("test content"), 0644)
			},
			expectFile:  false,
			expectError: false,
		},
		{
			name:    "no AGENTS.md file",
			enabled: true,
			setupFunc: func(dir string) error {
				return nil // no file creation
			},
			expectFile:  false,
			expectError: false,
		},
		{
			name:    "AGENTS.md file exists",
			enabled: true,
			setupFunc: func(dir string) error {
				content := "# Project Agents\n\nThis is a test project with specific instructions."
				return os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644)
			},
			expectFile:      true,
			expectError:     false,
			expectedPath:    "AGENTS.md",
			expectedContent: "# Project Agents\n\nThis is a test project with specific instructions.",
		},
		{
			name:    "case insensitive detection",
			enabled: true,
			setupFunc: func(dir string) error {
				content := "Case insensitive test"
				return os.WriteFile(filepath.Join(dir, "agents.md"), []byte(content), 0644)
			},
			expectFile:      true,
			expectError:     false,
			expectedPath:    "agents.md",
			expectedContent: "Case insensitive test",
		},
		{
			name:    "mixed case detection",
			enabled: true,
			setupFunc: func(dir string) error {
				content := "Mixed case test"
				return os.WriteFile(filepath.Join(dir, "Agents.MD"), []byte(content), 0644)
			},
			expectFile:      true,
			expectError:     false,
			expectedPath:    "Agents.MD",
			expectedContent: "Mixed case test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "agents-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Setup test files
			if err := tt.setupFunc(tmpDir); err != nil {
				t.Fatalf("Failed to setup test: %v", err)
			}

			// Create detector and test
			detector := NewDetector(tt.enabled)
			agentsFile, err := detector.DetectInDirectory(tmpDir)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check file expectation
			if tt.expectFile && agentsFile == nil {
				t.Error("Expected agents file but got nil")
			}
			if !tt.expectFile && agentsFile != nil {
				t.Error("Expected no agents file but got one")
			}

			// Check file content if expected
			if tt.expectFile && agentsFile != nil {
				if !strings.HasSuffix(agentsFile.Path, tt.expectedPath) {
					t.Errorf("Expected path to end with %s, got %s", tt.expectedPath, agentsFile.Path)
				}
				if agentsFile.Content != tt.expectedContent {
					t.Errorf("Expected content %q, got %q", tt.expectedContent, agentsFile.Content)
				}
				if agentsFile.Directory != tmpDir {
					t.Errorf("Expected directory %s, got %s", tmpDir, agentsFile.Directory)
				}
			}
		})
	}
}

func TestDetector_DetectInWorkingDirectory(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		setupFunc  func() (func(), error) // returns cleanup function and error
		expectFile bool
	}{
		{
			name:    "detector disabled",
			enabled: false,
			setupFunc: func() (func(), error) {
				// Create AGENTS.md in current working directory
				content := "test content"
				if err := os.WriteFile("AGENTS.md", []byte(content), 0644); err != nil {
					return nil, err
				}
				return func() { os.Remove("AGENTS.md") }, nil
			},
			expectFile: false,
		},
		{
			name:    "AGENTS.md in working directory",
			enabled: true,
			setupFunc: func() (func(), error) {
				content := "working directory test"
				if err := os.WriteFile("AGENTS.md", []byte(content), 0644); err != nil {
					return nil, err
				}
				return func() { os.Remove("AGENTS.md") }, nil
			},
			expectFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cleanup, err := tt.setupFunc()
			if err != nil {
				t.Fatalf("Failed to setup test: %v", err)
			}
			defer cleanup()

			// Test
			detector := NewDetector(tt.enabled)
			agentsFile, err := detector.DetectInWorkingDirectory()

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectFile && agentsFile == nil {
				t.Error("Expected agents file but got nil")
			}
			if !tt.expectFile && agentsFile != nil {
				t.Error("Expected no agents file but got one")
			}
		})
	}
}

func TestAgentsFile_FormatAsSystemPromptAddition(t *testing.T) {
	tests := []struct {
		name       string
		agentsFile *AgentsFile
		expected   string
	}{
		{
			name:       "nil agents file",
			agentsFile: nil,
			expected:   "",
		},
		{
			name: "valid agents file",
			agentsFile: &AgentsFile{
				Path:      "/test/path/AGENTS.md",
				Content:   "Test project instructions\nWith multiple lines",
				Directory: "/test/path",
			},
			expected: "\n\n--- PROJECT CONTEXT (from AGENTS.md) ---\nWorking directory: /test/path\nProject-specific instructions:\n\nTest project instructions\nWith multiple lines\n--- END PROJECT CONTEXT ---",
		},
		{
			name: "empty content",
			agentsFile: &AgentsFile{
				Path:      "/empty/AGENTS.md",
				Content:   "",
				Directory: "/empty",
			},
			expected: "\n\n--- PROJECT CONTEXT (from AGENTS.md) ---\nWorking directory: /empty\nProject-specific instructions:\n\n\n--- END PROJECT CONTEXT ---",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.agentsFile.FormatAsSystemPromptAddition()
			if result != tt.expected {
				t.Errorf("FormatAsSystemPromptAddition() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestAgentsFile_GetSummary(t *testing.T) {
	tests := []struct {
		name       string
		agentsFile *AgentsFile
		expected   string
	}{
		{
			name:       "nil agents file",
			agentsFile: nil,
			expected:   "No AGENTS.md file detected",
		},
		{
			name: "file with content",
			agentsFile: &AgentsFile{
				Path:      "/test/AGENTS.md",
				Content:   "# Project\n\nThis is a test project with instructions.\nMore details here.",
				Directory: "/test",
			},
			expected: "AGENTS.md detected: 4 lines, 71 chars\nDirectory: /test\nPreview: This is a test project with instructions.",
		},
		{
			name: "file with long first line",
			agentsFile: &AgentsFile{
				Path:      "/test/AGENTS.md",
				Content:   "# Header\n\nThis is a very long line that should be truncated because it exceeds the 50 character limit set in the function",
				Directory: "/test",
			},
			expected: "AGENTS.md detected: 3 lines, 121 chars\nDirectory: /test\nPreview: This is a very long line that should be truncat...",
		},
		{
			name: "file with only headers",
			agentsFile: &AgentsFile{
				Path:      "/test/AGENTS.md",
				Content:   "# Main Header\n## Sub Header\n### Another Header",
				Directory: "/test",
			},
			expected: "AGENTS.md detected: 3 lines, 46 chars\nDirectory: /test\nPreview: No content preview available",
		},
		{
			name: "empty file",
			agentsFile: &AgentsFile{
				Path:      "/test/AGENTS.md",
				Content:   "",
				Directory: "/test",
			},
			expected: "AGENTS.md detected: 1 lines, 0 chars\nDirectory: /test\nPreview: No content preview available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.agentsFile.GetSummary()
			if result != tt.expected {
				t.Errorf("GetSummary() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDetector_LoadAgentsFile(t *testing.T) {
	// Create temporary directory and file
	tmpDir, err := os.MkdirTemp("", "agents-load-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := "# Test AGENTS.md\n\nThis is test content for loading."
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	detector := NewDetector(true)
	agentsFile, err := detector.loadAgentsFile(agentsPath, tmpDir)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if agentsFile == nil {
		t.Fatal("Expected agents file but got nil")
	}
	if agentsFile.Path != agentsPath {
		t.Errorf("Expected path %s, got %s", agentsPath, agentsFile.Path)
	}
	if agentsFile.Content != content {
		t.Errorf("Expected content %q, got %q", content, agentsFile.Content)
	}
	if agentsFile.Directory != tmpDir {
		t.Errorf("Expected directory %s, got %s", tmpDir, agentsFile.Directory)
	}
}

func TestDetector_LoadAgentsFile_NonExistent(t *testing.T) {
	detector := NewDetector(true)
	agentsFile, err := detector.loadAgentsFile("/non/existent/path", "/non/existent")

	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if agentsFile != nil {
		t.Error("Expected nil agents file for non-existent file")
	}
}

// Benchmark tests for performance validation
func BenchmarkDetector_DetectInDirectory(b *testing.B) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "agents-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := "# Benchmark Test\n\nThis is a test file for benchmarking the detection performance."
	if err := os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte(content), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	detector := NewDetector(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := detector.DetectInDirectory(tmpDir)
		if err != nil {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkAgentsFile_FormatAsSystemPromptAddition(b *testing.B) {
	agentsFile := &AgentsFile{
		Path:      "/test/AGENTS.md",
		Content:   strings.Repeat("This is a line of test content.\n", 100), // ~3KB content
		Directory: "/test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agentsFile.FormatAsSystemPromptAddition()
	}
}
