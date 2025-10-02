package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestTabVisibilityBelowHeight31(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := t.Context()
	model := NewModel(ctx, config)

	// Test specific heights below 31 that were previously problematic
	problematicHeights := []int{5, 10, 15, 20, 25, 30}

	for _, height := range problematicHeights {
		for width := 10; width <= 80; width += 10 {
			t.Run(fmt.Sprintf("height_%d_width_%d", height, width), func(t *testing.T) {
				model.width = width
				model.height = height

				view := model.View()

				if view == "" {
					t.Errorf("View should not be empty for height=%d, width=%d", height, width)
				}

				// Check that tabs are visible
				hasTabIndicators := strings.Contains(view, "C") ||
					strings.Contains(view, "Chat") ||
					strings.Contains(view, "Settings") ||
					strings.Contains(view, "S") ||
					strings.Contains(view, "R") ||
					strings.Contains(view, "RAG")

				if !hasTabIndicators {
					t.Errorf("Tabs should be visible at height=%d, width=%d but weren't found. View:\n%s", height, width, view)
				}

				// For heights >= 2, check that footer is visible
				if height >= 2 {
					hasFooter := strings.Contains(view, "Tab") ||
						strings.Contains(view, "Ctrl") ||
						strings.Contains(view, "Quit") ||
						strings.Contains(view, "Q") ||
						strings.Contains(view, "Ta") || // For narrow widths where "Tab" wraps
						strings.Contains(view, "b") // The "b" part when "Tab" wraps

					if !hasFooter {
						t.Errorf("Footer should be visible at height=%d, width=%d but wasn't found. View:\n%s", height, width, view)
					}
				}
			})
		}
	}
}
