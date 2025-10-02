package logging

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Level != LevelInfo {
		t.Errorf("Expected default level to be Info, got %v", config.Level)
	}

	if !config.EnableFile {
		t.Error("Expected file logging to be enabled by default")
	}

	if config.LogDir == "" {
		t.Error("Expected default log directory to be set")
	}
}

func TestGetDefaultLogDir(t *testing.T) {
	logDir := DefaultDir()

	if logDir == "" {
		t.Error("Expected log directory to be non-empty")
	}

	// Should contain gollama-chat in the path
	if !strings.Contains(logDir, "gollama-chat") {
		t.Errorf("Expected log directory to contain 'gollama-chat', got %s", logDir)
	}

	// Platform-specific path ending validation
	switch runtime.GOOS {
	case "darwin":
		// macOS: should end with "gollama-chat" (~/Library/Logs/gollama-chat)
		if !strings.HasSuffix(logDir, "gollama-chat") {
			t.Errorf("Expected macOS log directory to end with 'gollama-chat', got %s", logDir)
		}
	default:
		// Linux, Windows, etc.: should end with "logs" (appname/logs)
		if !strings.HasSuffix(logDir, "logs") {
			t.Errorf("Expected log directory to end with 'logs', got %s", logDir)
		}
	}
}

func TestInitialize(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	config := &Config{
		Level:      LevelDebug,
		EnableFile: true,
		LogDir:     filepath.Join(tempDir, "test-logs"),
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	defer Close()

	// Check that log directory was created
	if _, err := os.Stat(config.LogDir); os.IsNotExist(err) {
		t.Errorf("Log directory was not created: %s", config.LogDir)
	}

	// Check that log file was created
	timestamp := time.Now().Format("2006-01-02")
	expectedLogFile := filepath.Join(config.LogDir, "gollama-chat-"+timestamp+".log")
	if _, err := os.Stat(expectedLogFile); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", expectedLogFile)
	}
}

func TestLogLevels(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	config := &Config{
		Level:      LevelWarn, // Set to warn level
		EnableFile: true,
		LogDir:     filepath.Join(tempDir, "test-logs"),
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	defer Close()

	logger := GetLogger()

	// Test logging at different levels
	logger.Debug("This debug message should not appear")
	logger.Info("This info message should not appear")
	logger.Warn("This warning message should appear")
	logger.Error("This error message should appear")

	// Read the log file to verify content
	timestamp := time.Now().Format("2006-01-02")
	logFile := filepath.Join(config.LogDir, "gollama-chat-"+timestamp+".log")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Should contain warn and error messages
	if !strings.Contains(logContent, "This warning message should appear") {
		t.Error("Warning message not found in log file")
	}

	if !strings.Contains(logContent, "This error message should appear") {
		t.Error("Error message not found in log file")
	}

	// Should not contain debug and info messages (due to log level)
	if strings.Contains(logContent, "This debug message should not appear") {
		t.Error("Debug message found in log file (should be filtered out)")
	}

	if strings.Contains(logContent, "This info message should not appear") {
		t.Error("Info message found in log file (should be filtered out)")
	}
}

func TestLoggerWithComponent(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	config := &Config{
		Level:      LevelDebug,
		EnableFile: true,
		LogDir:     filepath.Join(tempDir, "test-logs"),
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	defer Close()

	logger := GetLogger().WithComponent("test-component")
	logger.Info("Test message with component")

	// Read the log file to verify component is included
	timestamp := time.Now().Format("2006-01-02")
	logFile := filepath.Join(config.LogDir, "gollama-chat-"+timestamp+".log")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Should contain component information
	if !strings.Contains(logContent, "test-component") {
		t.Error("Component information not found in log file")
	}
}

func TestGlobalLoggerFunctions(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	config := &Config{
		Level:      LevelDebug,
		EnableFile: true,
		LogDir:     filepath.Join(tempDir, "test-logs"),
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	defer Close()

	// Test global logging functions
	Debug("Global debug message")
	Info("Global info message")
	Warn("Global warn message")
	Error("Global error message")

	// Read the log file to verify content
	timestamp := time.Now().Format("2006-01-02")
	logFile := filepath.Join(config.LogDir, "gollama-chat-"+timestamp+".log")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Should contain all messages
	expectedMessages := []string{
		"Global debug message",
		"Global info message",
		"Global warn message",
		"Global error message",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(logContent, msg) {
			t.Errorf("Message '%s' not found in log file", msg)
		}
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LogLevel(999), "UNKNOWN"},
	}

	for _, test := range tests {
		if got := test.level.String(); got != test.expected {
			t.Errorf("LogLevel(%d).String() = %s, expected %s", test.level, got, test.expected)
		}
	}
}
