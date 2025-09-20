package util

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestIsVisibleASCII(t *testing.T) {
	tests := []struct {
		name     string
		keyMsg   tea.KeyMsg
		expected bool
	}{
		{
			name:     "space character",
			keyMsg:   tea.KeyMsg{Type: tea.KeySpace},
			expected: true,
		},
		{
			name:     "letter a",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
			expected: true,
		},
		{
			name:     "letter Z",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Z'}},
			expected: true,
		},
		{
			name:     "digit 0",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}},
			expected: true,
		},
		{
			name:     "digit 9",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}},
			expected: true,
		},
		{
			name:     "exclamation mark",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}},
			expected: true,
		},
		{
			name:     "tilde",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'~'}},
			expected: true,
		},
		{
			name:     "at symbol",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}},
			expected: true,
		},
		{
			name:     "hash symbol",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'#'}},
			expected: true,
		},
		{
			name:     "enter key",
			keyMsg:   tea.KeyMsg{Type: tea.KeyEnter},
			expected: false,
		},
		{
			name:     "backspace key",
			keyMsg:   tea.KeyMsg{Type: tea.KeyBackspace},
			expected: false,
		},
		{
			name:     "escape key",
			keyMsg:   tea.KeyMsg{Type: tea.KeyEscape},
			expected: false,
		},
		{
			name:     "tab key",
			keyMsg:   tea.KeyMsg{Type: tea.KeyTab},
			expected: false,
		},
		{
			name:     "arrow up",
			keyMsg:   tea.KeyMsg{Type: tea.KeyUp},
			expected: false,
		},
		{
			name:     "arrow down",
			keyMsg:   tea.KeyMsg{Type: tea.KeyDown},
			expected: false,
		},
		{
			name:     "ctrl+c",
			keyMsg:   tea.KeyMsg{Type: tea.KeyCtrlC},
			expected: false,
		},
		{
			name:     "ctrl+d",
			keyMsg:   tea.KeyMsg{Type: tea.KeyCtrlD},
			expected: false,
		},
		// Edge cases with unicode/multi-byte characters (should return false)
		{
			name:     "unicode character",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'π'}},
			expected: false,
		},
		{
			name:     "emoji",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'😀'}},
			expected: false,
		},
		// Control characters (should return false)
		{
			name:     "null character",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0}},
			expected: false,
		},
		{
			name:     "bell character",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{7}},
			expected: false,
		},
		{
			name:     "newline character",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{10}},
			expected: false,
		},
		{
			name:     "carriage return",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{13}},
			expected: false,
		},
		// DEL character (ASCII 127) should return false
		{
			name:     "delete character",
			keyMsg:   tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{127}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVisibleASCII(tt.keyMsg)
			if result != tt.expected {
				t.Errorf("IsVisibleASCII() = %v, expected %v for key %v", result, tt.expected, tt.keyMsg.String())
			}
		})
	}
}

// Test edge cases and boundary conditions
func TestIsVisibleASCII_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name     string
		ascii    byte
		expected bool
	}{
		{"ASCII 32 (space)", 32, true},  // Space should return true (handled specially)
		{"ASCII 33 (!)", 33, true},      // First printable non-space character
		{"ASCII 126 (~)", 126, true},    // Last printable character
		{"ASCII 31", 31, false},         // Below printable range
		{"ASCII 127 (DEL)", 127, false}, // Above printable range
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a KeyMsg with the specific ASCII character
			var keyMsg tea.KeyMsg
			if tt.ascii == 32 {
				keyMsg = tea.KeyMsg{Type: tea.KeySpace}
			} else {
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(tt.ascii)}}
			}

			result := IsVisibleASCII(keyMsg)
			if result != tt.expected {
				t.Errorf("IsVisibleASCII() = %v, expected %v for ASCII %d", result, tt.expected, tt.ascii)
			}
		})
	}
}
