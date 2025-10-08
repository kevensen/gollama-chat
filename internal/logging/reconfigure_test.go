package logging

import (
	"testing"
)

func TestReconfigure(t *testing.T) {
	// Test that we can reconfigure the logging system
	config1 := &Config{
		Level:        LevelInfo,
		EnableFile:   false,
		EnableStderr: true,
	}

	if err := Initialize(config1); err != nil {
		t.Fatalf("Failed to initialize logging: %v", err)
	}

	// Verify initial level
	logger := GetLogger()
	if logger.level != LevelInfo {
		t.Errorf("Expected level %v, got %v", LevelInfo, logger.level)
	}

	// Reconfigure with debug level
	config2 := &Config{
		Level:        LevelDebug,
		EnableFile:   false,
		EnableStderr: true,
	}

	if err := Reconfigure(config2); err != nil {
		t.Fatalf("Failed to reconfigure logging: %v", err)
	}

	// Verify new level
	logger = GetLogger()
	if logger.level != LevelDebug {
		t.Errorf("Expected level %v after reconfigure, got %v", LevelDebug, logger.level)
	}

	// Clean up
	Close()
}
