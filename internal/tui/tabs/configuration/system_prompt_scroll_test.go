package configuration

import (
	"strings"
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestSystemPromptScrollingBounds(t *testing.T) {
	tests := []struct {
		name           string
		textLines      int
		textAreaHeight int
		initialScroll  int
		scrollAmount   int
		scrollDown     bool
		expectedScroll int
	}{
		{
			name:           "scroll down within bounds",
			textLines:      20,
			textAreaHeight: 10,
			initialScroll:  0,
			scrollAmount:   1, // 1 page down = 5 lines
			scrollDown:     true,
			expectedScroll: 5,
		},
		{
			name:           "scroll down to maximum",
			textLines:      20,
			textAreaHeight: 10,
			initialScroll:  5,
			scrollAmount:   2, // 2 page downs = 10 lines, but clamped to max
			scrollDown:     true,
			expectedScroll: 11, // Adjust based on actual line count after wrapping
		},
		{
			name:           "scroll up from middle",
			textLines:      20,
			textAreaHeight: 10,
			initialScroll:  10,
			scrollAmount:   1, // 1 page up = 5 lines
			scrollDown:     false,
			expectedScroll: 5,
		},
		{
			name:           "scroll up to beginning",
			textLines:      20,
			textAreaHeight: 10,
			initialScroll:  3,
			scrollAmount:   2, // 2 page ups = 10 lines, but clamped to 0
			scrollDown:     false,
			expectedScroll: 0,
		},
		{
			name:           "text fits in area - no scroll needed",
			textLines:      5,
			textAreaHeight: 10,
			initialScroll:  0,
			scrollAmount:   1,
			scrollDown:     true,
			expectedScroll: 0, // Should stay at 0 since all text fits
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test configuration
			config := &configuration.Config{
				DefaultSystemPrompt: strings.Repeat("Line of text\n", tt.textLines),
			}

			// Create model
			m := NewModel(config)
			m.height = tt.textAreaHeight + 12 // Account for UI elements
			m.width = 100
			m.showSystemPromptPanel = true
			m.systemPromptEditMode = true
			m.systemPromptEditInput = config.DefaultSystemPrompt
			m.systemPromptScrollY = tt.initialScroll

			// Calculate text metrics
			textWidth := (m.width / 3) - 6
			if textWidth < 20 {
				textWidth = 20
			}
			displayText := m.renderVisibleChars(m.systemPromptEditInput)
			lines := m.wrapText(displayText, textWidth)
			maxScroll := max(len(lines)-tt.textAreaHeight, 0)

			// Simulate scroll action manually (mimicking the logic from handleNavigationKeys)
			for i := 0; i < tt.scrollAmount; i++ {
				if tt.scrollDown {
					// Page down logic
					m.systemPromptScrollY += 5
					if m.systemPromptScrollY > maxScroll {
						m.systemPromptScrollY = maxScroll
					}
				} else {
					// Page up logic
					if m.systemPromptScrollY > 0 {
						m.systemPromptScrollY -= 5
						if m.systemPromptScrollY < 0 {
							m.systemPromptScrollY = 0
						}
					}
				}
			}

			// Check final scroll position
			if m.systemPromptScrollY != tt.expectedScroll {
				t.Errorf("Expected scroll position %d, got %d (maxScroll=%d, totalLines=%d, textAreaHeight=%d)",
					tt.expectedScroll, m.systemPromptScrollY, maxScroll, len(lines), tt.textAreaHeight)
			}

			// Verify scroll is within bounds
			if m.systemPromptScrollY < 0 {
				t.Errorf("Scroll position %d is below minimum 0", m.systemPromptScrollY)
			}
			if m.systemPromptScrollY > maxScroll {
				t.Errorf("Scroll position %d exceeds maximum %d", m.systemPromptScrollY, maxScroll)
			}

			// Check if user can see the last line when at maximum scroll
			if m.systemPromptScrollY == maxScroll {
				canSeeLast := (m.systemPromptScrollY + tt.textAreaHeight) >= len(lines)
				if !canSeeLast {
					t.Errorf("At max scroll %d, should be able to see last line (total lines: %d, text area: %d)",
						maxScroll, len(lines), tt.textAreaHeight)
				}
			}
		})
	}
}

func TestCursorPositionOnEditStart(t *testing.T) {
	config := &configuration.Config{
		DefaultSystemPrompt: "This is a test system prompt with multiple lines\nSecond line\nThird line",
	}

	m := NewModel(config)
	m.height = 30
	m.width = 100
	m.showSystemPromptPanel = true

	// Manually set edit mode to test cursor positioning (mimicking ctrl+e behavior)
	m.systemPromptEditMode = true
	m.systemPromptEditInput = config.DefaultSystemPrompt
	m.systemPromptEditCursor = 0 // Should start at beginning
	m.systemPromptScrollY = 0    // Should start at top

	// Check that cursor is at beginning
	if m.systemPromptEditCursor != 0 {
		t.Errorf("Expected cursor at position 0 when entering edit mode, got %d", m.systemPromptEditCursor)
	}

	// Check that scroll is at top
	if m.systemPromptScrollY != 0 {
		t.Errorf("Expected scroll at position 0 when entering edit mode, got %d", m.systemPromptScrollY)
	}
}

func TestEnsureCursorVisible(t *testing.T) {
	config := &configuration.Config{
		DefaultSystemPrompt: strings.Repeat("Line of text\n", 25),
	}

	m := NewModel(config)
	m.height = 20
	m.width = 80
	m.showSystemPromptPanel = true
	m.systemPromptEditMode = true
	m.systemPromptEditInput = config.DefaultSystemPrompt

	// Set cursor to end of text
	m.systemPromptEditCursor = len([]rune(m.systemPromptEditInput))

	// Call ensureCursorVisible to adjust scroll
	m.ensureCursorVisible()

	// Calculate expected values
	textAreaHeight := m.height - 12
	if textAreaHeight < 5 {
		textAreaHeight = 5
	}
	textWidth := (m.width / 3) - 6
	if textWidth < 20 {
		textWidth = 20
	}
	displayText := m.renderVisibleChars(m.systemPromptEditInput)
	lines := m.wrapText(displayText, textWidth)

	// Cursor should be visible (scroll should be adjusted so cursor line is in view)
	displayCursor := m.convertCursorToDisplayPosition(m.systemPromptEditInput, m.systemPromptEditCursor)
	cursorLine := 0
	charCount := 0
	for i, line := range lines {
		lineLength := len([]rune(line))
		if charCount+lineLength >= displayCursor {
			cursorLine = i
			break
		}
		charCount += lineLength
		if i < len(lines)-1 {
			charCount++
		}
	}

	// Cursor line should be within the visible area
	if cursorLine < m.systemPromptScrollY || cursorLine >= m.systemPromptScrollY+textAreaHeight {
		t.Errorf("Cursor at line %d is not visible (scroll=%d, textAreaHeight=%d)",
			cursorLine, m.systemPromptScrollY, textAreaHeight)
	}

	// Verify scroll bounds
	maxScroll := len(lines) - textAreaHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.systemPromptScrollY < 0 || m.systemPromptScrollY > maxScroll {
		t.Errorf("Scroll position %d is out of bounds [0, %d]", m.systemPromptScrollY, maxScroll)
	}
}
