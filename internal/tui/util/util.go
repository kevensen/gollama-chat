package util

import (
	tea "github.com/charmbracelet/bubbletea"
)

// IsVisibleASCII checks if the given tea.KeyMsg represents a visible ASCII character
// including the space bar (ASCII 32-126)
func IsVisibleASCII(keyMsg tea.KeyMsg) bool {
	// Get the key string representation
	key := keyMsg.String()

	// Handle space bar specifically
	if key == " " {
		return true
	}

	// Check if it's a single visible ASCII character (printable characters 33-126)
	// We exclude space (32) here since we handle it above
	if len(key) == 1 {
		char := key[0]
		return char >= 33 && char <= 126
	}

	return false
}
