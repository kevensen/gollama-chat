package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevensen/gollama-chat/internal/logging"
)

// AgentsFile represents information about a detected AGENTS.md file
type AgentsFile struct {
	Path      string // Absolute path to the AGENTS.md file
	Content   string // Content of the file
	Directory string // Directory containing the file
}

// Detector handles detection and loading of AGENTS.md files
type Detector struct {
	enabled bool
	logger  *logging.Logger
}

// NewDetector creates a new AGENTS.md detector
func NewDetector(enabled bool) *Detector {
	return &Detector{
		enabled: enabled,
		logger:  logging.WithComponent("agents-detector"),
	}
}

// SetEnabled enables or disables AGENTS.md detection
func (d *Detector) SetEnabled(enabled bool) {
	d.enabled = enabled
	if enabled {
		d.logger.Info("AGENTS.md detection enabled")
	} else {
		d.logger.Info("AGENTS.md detection disabled")
	}
}

// IsEnabled returns whether AGENTS.md detection is enabled
func (d *Detector) IsEnabled() bool {
	return d.enabled
}

// DetectInWorkingDirectory looks for an AGENTS.md file in the current working directory
func (d *Detector) DetectInWorkingDirectory() (*AgentsFile, error) {
	if !d.enabled {
		d.logger.Debug("AGENTS.md detection is disabled")
		return nil, nil
	}

	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		d.logger.Error("Failed to get working directory", "error", err)
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	return d.DetectInDirectory(wd)
}

// DetectInDirectory looks for an AGENTS.md file in the specified directory
func (d *Detector) DetectInDirectory(dir string) (*AgentsFile, error) {
	if !d.enabled {
		d.logger.Debug("AGENTS.md detection is disabled")
		return nil, nil
	}

	d.logger.Debug("Checking for AGENTS.md file", "directory", dir)

	// Always do a directory scan to find the actual filename
	// This ensures we get the correct case-preserved filename on all filesystems
	entries, err := os.ReadDir(dir)
	if err != nil {
		d.logger.Debug("Failed to read directory", "directory", dir, "error", err)
		return nil, nil // Return nil instead of error - file just doesn't exist
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.EqualFold(name, "AGENTS.md") {
			foundPath := filepath.Join(dir, name)
			d.logger.Info("Found agents file", "path", foundPath, "actual_name", name)
			return d.loadAgentsFile(foundPath, dir)
		}
	}

	d.logger.Debug("No AGENTS.md file found", "directory", dir)
	return nil, nil
}

// loadAgentsFile reads and loads the content of an AGENTS.md file
func (d *Detector) loadAgentsFile(path string, dir string) (*AgentsFile, error) {
	d.logger.Debug("Loading AGENTS.md file", "path", path)

	content, err := os.ReadFile(path)
	if err != nil {
		d.logger.Error("Failed to read AGENTS.md file", "path", path, "error", err)
		return nil, fmt.Errorf("failed to read AGENTS.md file: %w", err)
	}

	contentStr := string(content)
	d.logger.Info("Successfully loaded AGENTS.md file",
		"path", path,
		"size_bytes", len(content),
		"size_chars", len(contentStr))

	return &AgentsFile{
		Path:      path,
		Content:   contentStr,
		Directory: dir,
	}, nil
}

// FormatAsSystemPromptAddition formats the AGENTS.md content as an addition to the system prompt
func (af *AgentsFile) FormatAsSystemPromptAddition() string {
	if af == nil {
		return ""
	}

	var builder strings.Builder

	builder.WriteString("\n\n--- PROJECT CONTEXT (from AGENTS.md) ---\n")
	builder.WriteString(fmt.Sprintf("Working directory: %s\n", af.Directory))
	builder.WriteString("Project-specific instructions:\n\n")
	builder.WriteString(af.Content)
	builder.WriteString("\n--- END PROJECT CONTEXT ---")

	return builder.String()
}

// GetSummary returns a brief summary of the agents file for display purposes
func (af *AgentsFile) GetSummary() string {
	if af == nil {
		return "No AGENTS.md file detected"
	}

	lines := strings.Split(af.Content, "\n")
	lineCount := len(lines)
	charCount := len(af.Content)

	// Get first non-empty line as a preview
	preview := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			if len(trimmed) > 50 {
				preview = trimmed[:47] + "..."
			} else {
				preview = trimmed
			}
			break
		}
	}

	if preview == "" {
		preview = "No content preview available"
	}

	return fmt.Sprintf("AGENTS.md detected: %d lines, %d chars\nDirectory: %s\nPreview: %s",
		lineCount, charCount, af.Directory, preview)
}
