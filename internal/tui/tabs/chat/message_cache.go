package chat

import (
	"strings"
)

// MessageCache stores precomputed message renders to avoid recalculations
type MessageCache struct {
	// Rendered messages cache
	renderedMessages map[string][]string

	// Last window width used for rendering
	lastWidth int

	// Flag to indicate if cache needs to be invalidated
	needsRefresh bool

	// The last computed total height of all messages
	cachedTotalHeight int
}

// NewMessageCache creates a new message cache
func NewMessageCache() *MessageCache {
	return &MessageCache{
		renderedMessages: make(map[string][]string),
		needsRefresh:     true,
	}
}

// GetRenderedMessage gets a precomputed message or computes and caches it
func (c *MessageCache) GetRenderedMessage(model *Model, msg Message, width int) []string {
	// Generate a cache key for the message
	key := msg.Role + msg.Time.String() + msg.Content

	// Check if width changed - if so we need to invalidate cache
	if width != c.lastWidth {
		c.lastWidth = width
		c.needsRefresh = true
		c.renderedMessages = make(map[string][]string)
	}

	// Return cached version if available
	if lines, ok := c.renderedMessages[key]; ok && !c.needsRefresh {
		return lines
	}

	// Compute the message rendering
	lines := model.formatMessage(msg)

	// Cache the result
	c.renderedMessages[key] = lines

	return lines
}

// InvalidateCache marks the cache for refresh
func (c *MessageCache) InvalidateCache() {
	c.needsRefresh = true
}

// GetTotalHeight gets the cached height or computes it
func (c *MessageCache) GetTotalHeight(model *Model) int {
	if !c.needsRefresh && c.cachedTotalHeight > 0 {
		return c.cachedTotalHeight
	}

	// Compute height of all messages
	height := 0
	for _, msg := range model.messages {
		renderedMsg := c.GetRenderedMessage(model, msg, model.width)
		height += len(renderedMsg)
	}

	c.cachedTotalHeight = height
	c.needsRefresh = false

	return height
}

// RenderAllMessages renders all messages with caching
func (c *MessageCache) RenderAllMessages(model *Model) string {
	// Calculate available height for messages, accounting for system prompt
	systemPromptHeight := model.getSystemPromptHeight()
	availableHeight := model.height - 6 - systemPromptHeight // Reserve space for input, status, and system prompt

	// Apply styling with border regardless of message count
	messageStyle := model.styles.messages.
		Width(model.width - 2). // Account for border width
		Height(availableHeight) // Use calculated available height

	if len(model.messages) == 0 {
		emptyStyle := model.styles.emptyMessages.
			Width(model.width - 4).     // Account for parent border and padding
			Height(availableHeight - 2) // Account for parent border and padding

		return messageStyle.Render(emptyStyle.Render("No messages yet. Type a message and press Enter to start chatting!"))
	}

	var allLines []string

	// Get all rendered message lines
	for _, msg := range model.messages {
		lines := c.GetRenderedMessage(model, msg, model.width-4) // Account for border width
		allLines = append(allLines, lines...)
	}

	// Calculate the visible portion based on scroll offset
	// Use the same availableHeight calculated above
	totalLines := len(allLines)

	// Apply scroll offset
	if totalLines <= availableHeight {
		// All lines fit, no scrolling needed
		model.scrollOffset = 0
	} else {
		// Get the visible slice of lines
		startIdx := model.scrollOffset
		endIdx := startIdx + availableHeight

		// Bounds checking
		if endIdx > totalLines {
			endIdx = totalLines
			startIdx = endIdx - availableHeight
			if startIdx < 0 {
				startIdx = 0
			}
		}

		allLines = allLines[startIdx:endIdx]
	}

	// Render final content with proper styling and ensure consistent dimensions
	content := strings.Join(allLines, "\n")

	// Use the pre-sized style created above
	return messageStyle.Render(content)
}
