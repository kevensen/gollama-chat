package chat

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		expectedMin int
		expectedMax int
		description string
	}{
		{
			name:        "empty string",
			text:        "",
			expectedMin: 0,
			expectedMax: 0,
			description: "Empty string should return 0 tokens",
		},
		{
			name:        "single word",
			text:        "hello",
			expectedMin: 1,
			expectedMax: 2,
			description: "Single word should return approximately 1-2 tokens",
		},
		{
			name:        "simple sentence",
			text:        "Hello world",
			expectedMin: 2,
			expectedMax: 4,
			description: "Simple sentence should return 2-4 tokens",
		},
		{
			name:        "very long text",
			text:        "This is a very long text that contains many words and should result in a proportionally higher token count when estimated using the approximation algorithm.",
			expectedMin: 30,
			expectedMax: 45,
			description: "Long text should scale appropriately",
		},
		// New edge cases
		{
			name:        "unicode characters",
			text:        "Hello 世界! 🌍 Testing unicode characters",
			expectedMin: 6,
			expectedMax: 12,
			description: "Unicode characters should be handled properly",
		},
		{
			name:        "code block",
			text:        "```go\nfunc main() {\n    fmt.Println(\"Hello World\")\n}\n```",
			expectedMin: 8,
			expectedMax: 16,
			description: "Code blocks should estimate tokens appropriately",
		},
		{
			name:        "markdown formatting",
			text:        "**bold text** and _italic text_ with `inline code` formatting",
			expectedMin: 8,
			expectedMax: 15,
			description: "Markdown formatting should not drastically affect token count",
		},
		{
			name:        "special characters and punctuation",
			text:        "Hello, world! How are you? I'm fine... Really? Yes!!! @#$%^&*()",
			expectedMin: 10,
			expectedMax: 20,
			description: "Special characters and punctuation should be handled",
		},
		{
			name:        "very long repeated text",
			text:        strings.Repeat("word ", 500),
			expectedMin: 580,
			expectedMax: 680,
			description: "Very long text should scale linearly",
		},
		{
			name:        "mixed content",
			text:        "System message: You are a helpful AI assistant. Code: `func test() {}`. Unicode: 世界 🚀",
			expectedMin: 15,
			expectedMax: 25,
			description: "Mixed content types should be estimated reasonably",
		},
		{
			name:        "whitespace only",
			text:        "   \n\t  \n  ",
			expectedMin: 0,
			expectedMax: 2,
			description: "Whitespace-only text should have minimal tokens",
		},
		{
			name:        "single character",
			text:        "a",
			expectedMin: 1,
			expectedMax: 1,
			description: "Single character should be 1 token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateTokens(tt.text)

			if result < tt.expectedMin || result > tt.expectedMax {
				t.Errorf("estimateTokens(%q) = %d, expected between %d and %d (description: %s)",
					tt.text, result, tt.expectedMin, tt.expectedMax, tt.description)
			}
		})
	}
}

func TestWrapText(t *testing.T) {
	// Create a minimal model for testing
	model := Model{}

	tests := []struct {
		name     string
		text     string
		width    int
		expected []string
	}{
		{
			name:     "empty text",
			text:     "",
			width:    10,
			expected: []string{""},
		},
		{
			name:     "single word within width",
			text:     "hello",
			width:    10,
			expected: []string{"hello"},
		},
		{
			name:     "multiple words within width",
			text:     "hello world",
			width:    20,
			expected: []string{"hello world"},
		},
		{
			name:     "text requiring wrapping",
			text:     "this is a long sentence that needs wrapping",
			width:    15,
			expected: []string{"this is a long", "sentence that", "needs wrapping"},
		},
		{
			name:     "single long word",
			text:     "supercalifragilisticexpialidocious",
			width:    10,
			expected: []string{"supercalifragilisticexpialidocious"}, // Long words don't get broken
		},
		{
			name:     "exact width match",
			text:     "exactly ten",
			width:    11,
			expected: []string{"exactly ten"},
		},
		{
			name:     "zero width",
			text:     "hello world",
			width:    0,
			expected: []string{"hello world"}, // Should return original text
		},
		{
			name:     "negative width",
			text:     "hello world",
			width:    -5,
			expected: []string{"hello world"}, // Should return original text
		},
		{
			name:     "multiple spaces",
			text:     "hello    world    test",
			width:    10,
			expected: []string{"hello", "world test"}, // Fields() handles multiple spaces
		},
		{
			name:     "text with newlines",
			text:     "hello\nworld test",
			width:    15,
			expected: []string{"hello", "world test"}, // Newlines are preserved as line breaks
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := model.wrapText(tt.text, tt.width)

			if len(result) != len(tt.expected) {
				t.Errorf("wrapText(%q, %d) returned %d lines, expected %d lines",
					tt.text, tt.width, len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Expected: %v", tt.expected)
				return
			}

			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("wrapText(%q, %d) line %d = %q, expected %q",
						tt.text, tt.width, i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestParseMarkdownFormatting(t *testing.T) {
	// Create a test model with styles
	model := Model{
		styles: DefaultStyles(),
	}

	tests := []struct {
		name     string
		input    string
		contains string // Check if the output contains bold styling
	}{
		{
			name:     "no markdown",
			input:    "This is plain text",
			contains: "This is plain text",
		},
		{
			name:     "single bold word",
			input:    "This is **bold** text",
			contains: "bold", // The word should be styled
		},
		{
			name:     "multiple bold sections",
			input:    "This has **multiple** bold **sections**",
			contains: "multiple", // Should contain both bold words
		},
		{
			name:     "bold at start",
			input:    "**Bold** at the beginning",
			contains: "Bold",
		},
		{
			name:     "bold at end",
			input:    "Text with **bold** at end",
			contains: "bold",
		},
		{
			name:     "unclosed bold markers",
			input:    "This has **unclosed markers",
			contains: "This has **unclosed markers", // Should remain unchanged
		},
		{
			name:     "empty bold markers",
			input:    "This has **** empty markers",
			contains: "", // Empty content between markers
		},
		{
			name:     "simple italic text",
			input:    "This is _italic_ text",
			contains: "italic", // Should contain the word "italic" without _
		},
		{
			name:     "multiple italic sections",
			input:    "This has _multiple_ italic _sections_",
			contains: "multiple", // Should process both italic sections
		},
		{
			name:     "italic at start",
			input:    "_Italic_ at the beginning",
			contains: "Italic",
		},
		{
			name:     "italic at end",
			input:    "Text with _italic_ at end",
			contains: "italic",
		},
		{
			name:     "unclosed italic markers",
			input:    "This has _unclosed markers",
			contains: "This has _unclosed markers", // Should remain unchanged
		},
		{
			name:     "empty italic markers",
			input:    "This has __ empty markers",
			contains: "", // Empty content between markers
		},
		{
			name:     "mixed bold and italic",
			input:    "This has **bold** and _italic_ text",
			contains: "bold", // Should contain both styled text
		},
		{
			name:     "bullet with italic text",
			input:    "* Alabama - _Montgomery_",
			contains: "Montgomery", // Should contain "Montgomery" without _
		},
		{
			name:     "bullet with mixed formatting",
			input:    "* **Alabama** - _Montgomery_",
			contains: "Alabama", // Should contain both "Alabama" and "Montgomery"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := model.parseMarkdownFormatting(tt.input)

			// For basic verification, check that the result is not empty (unless expected)
			if len(tt.input) > 0 && len(result) == 0 {
				t.Errorf("parseMarkdownFormatting(%q) returned empty string", tt.input)
			}

			// Check that the result contains expected content
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("parseMarkdownFormatting(%q) result should contain %q, got: %q",
					tt.input, tt.contains, result)
			}
		})
	}
}

func TestWrapRegularTextWithMarkdown(t *testing.T) {
	// Create a test model with styles
	model := Model{
		styles: DefaultStyles(),
		width:  20, // Set a specific width for testing
	}

	tests := []struct {
		name     string
		input    string
		width    int
		minLines int // Minimum expected lines
	}{
		{
			name:     "short text with bold",
			input:    "**Bold** text",
			width:    20,
			minLines: 1,
		},
		{
			name:     "long text with bold that wraps",
			input:    "This is a very long line with **bold** text that should wrap",
			width:    20,
			minLines: 3,
		},
		{
			name:     "multiple bold sections",
			input:    "Text with **multiple** bold **sections** here",
			width:    15,
			minLines: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := model.wrapRegularText(tt.input, tt.width)

			if len(result) < tt.minLines {
				t.Errorf("wrapRegularText(%q, %d) returned %d lines, expected at least %d",
					tt.input, tt.width, len(result), tt.minLines)
			}

			// Verify that all lines are non-empty (unless the input was empty)
			for i, line := range result {
				if len(tt.input) > 0 && strings.TrimSpace(line) == "" && len(result) == 1 {
					t.Errorf("wrapRegularText(%q, %d) line %d is empty", tt.input, tt.width, i)
				}
			}
		})
	}
}

// TestBulletFormattingFullPipeline tests the complete bullet formatting pipeline
func TestBulletFormattingFullPipeline(t *testing.T) {
	model := Model{
		styles: DefaultStyles(),
		width:  50,
	}

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "bullet with bold",
			input: "* item that should be **bold**.",
		},
		{
			name:  "bullet with bold at start",
			input: "* **Another bold** item",
		},
		{
			name:  "bullet with italic",
			input: "* Alabama - _Montgomery_",
		},
		{
			name:  "bullet with mixed formatting",
			input: "* **Alabama** - _Montgomery_",
		},
		{
			name:  "multiple states with mixed formatting",
			input: "* **Alabama** - _Montgomery_\n* **Alaska** - _Juneau_",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Test the full formatting pipeline
			result := model.formatTextWithMarkdown(tt.input, 50)

			t.Logf("Full pipeline result for %q:", tt.input)
			for i, line := range result {
				t.Logf("  Line %d: %q", i, line)

				// Check that ** markers are processed out
				if strings.Contains(line, "**") {
					t.Errorf("Line %d still contains ** markers: %q", i, line)
				}

				// Check that _ markers are processed out
				if strings.Contains(line, "_") {
					t.Errorf("Line %d still contains _ markers: %q", i, line)
				}
			}
		})
	}
}

// TestBoldStylingApplied checks if bold styling (ANSI codes) is actually applied
func TestBoldStylingApplied(t *testing.T) {
	model := Model{
		styles: DefaultStyles(),
	}

	// Test parseMarkdownFormatting directly to see styling
	input := "* item that should be **bold**."
	result := model.parseMarkdownFormatting(input)

	t.Logf("Input: %q", input)
	t.Logf("Output: %q", result)
	t.Logf("Output length: %d", len(result))

	// The result should be longer than the input without ** because of ANSI escape codes
	expectedMinLength := len(input) - 4 // Remove the 4 ** characters
	if len(result) < expectedMinLength {
		t.Errorf("Expected output to be at least %d characters (for styling), got %d", expectedMinLength, len(result))
	}

	// Check that ** markers are removed
	if strings.Contains(result, "**") {
		t.Errorf("Output still contains ** markers: %q", result)
	}

	// Check that "bold" text is still present
	if !strings.Contains(result, "bold") {
		t.Errorf("Output should contain 'bold' text: %q", result)
	}
}

// TestItalicStylingApplied checks if italic styling (ANSI codes) is actually applied
func TestItalicStylingApplied(t *testing.T) {
	model := Model{
		styles: DefaultStyles(),
	}

	// Test parseMarkdownFormatting directly to see styling
	input := "* Alabama - _Montgomery_"
	result := model.parseMarkdownFormatting(input)

	t.Logf("Input: %q", input)
	t.Logf("Output: %q", result)
	t.Logf("Output length: %d", len(result))

	// The result should be longer than the input without _ because of ANSI escape codes
	expectedMinLength := len(input) - 2 // Remove the 2 _ characters
	if len(result) < expectedMinLength {
		t.Errorf("Expected output to be at least %d characters (for styling), got %d", expectedMinLength, len(result))
	}

	// Check that _ markers are removed
	if strings.Contains(result, "_") {
		t.Errorf("Output still contains _ markers: %q", result)
	}

	// Check that "Montgomery" text is still present
	if !strings.Contains(result, "Montgomery") {
		t.Errorf("Output should contain 'Montgomery' text: %q", result)
	}
}
