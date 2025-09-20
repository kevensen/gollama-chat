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
			expected: []string{"hello world", "test"}, // Fields() handles newlines
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
