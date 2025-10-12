package agents

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDetector_CrossPlatformCaseSensitivity tests case sensitivity behavior across different platforms
func TestDetector_CrossPlatformCaseSensitivity(t *testing.T) {
	tests := []struct {
		name           string
		createFilename string
		description    string
	}{
		{"lowercase", "agents.md", "Test lowercase filename detection"},
		{"uppercase", "AGENTS.MD", "Test uppercase filename detection"},
		{"mixed_case", "Agents.Md", "Test mixed case filename detection"},
		{"standard", "AGENTS.md", "Test standard case filename detection"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "agents-cross-platform-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			// Create file with specific case
			content := "Cross-platform test content"
			filePath := filepath.Join(tmpDir, tt.createFilename)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test detection
			detector := NewDetector(true)
			agentsFile, err := detector.DetectInDirectory(tmpDir)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if agentsFile == nil {
				t.Fatal("Expected agents file but got nil")
			}

			// Verify the returned path uses the actual filename from the filesystem
			actualFilename := filepath.Base(agentsFile.Path)
			if actualFilename != tt.createFilename {
				t.Errorf("Expected filename %s, got %s (OS: %s)", tt.createFilename, actualFilename, runtime.GOOS)
			}

			// Verify content is correct
			if agentsFile.Content != content {
				t.Errorf("Expected content %q, got %q", content, agentsFile.Content)
			}

			// Log platform-specific behavior for debugging
			t.Logf("Platform: %s, Created: %s, Detected: %s", runtime.GOOS, tt.createFilename, actualFilename)
		})
	}
}

// TestDetector_FileSystemCaseHandling tests how we handle different filesystem case behaviors
func TestDetector_FileSystemCaseHandling(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "agents-filesystem-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a file with lowercase name
	content := "Filesystem case handling test"
	actualFile := filepath.Join(tmpDir, "agents.md")
	if err := os.WriteFile(actualFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test that our detector finds the file with the correct actual filename
	detector := NewDetector(true)
	agentsFile, err := detector.DetectInDirectory(tmpDir)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if agentsFile == nil {
		t.Fatal("Expected agents file but got nil")
	}

	// The key test: verify we get the actual filename as stored on the filesystem
	actualBasename := filepath.Base(agentsFile.Path)
	expectedBasename := "agents.md"

	if actualBasename != expectedBasename {
		t.Errorf("Expected basename %s, got %s", expectedBasename, actualBasename)
	}

	// Verify the file can actually be read using the returned path
	readContent, err := os.ReadFile(agentsFile.Path)
	if err != nil {
		t.Errorf("Failed to read file using returned path: %v", err)
	}
	if string(readContent) != content {
		t.Errorf("Content mismatch when reading via returned path")
	}

	t.Logf("Platform: %s, File system preserved case correctly", runtime.GOOS)
}
