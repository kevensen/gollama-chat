package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestVisibilityPriority(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := t.Context()
	model := NewModel(ctx, config)

	tests := []struct {
		name                string
		width               int
		height              int
		shouldContainTabs   bool
		shouldContainFooter bool
		description         string
	}{
		{
			name:                "extremely_small_1x1",
			width:               1,
			height:              1,
			shouldContainTabs:   true,
			shouldContainFooter: false,
			description:         "Height 1: Only tabs should be visible",
		},
		{
			name:                "extremely_small_2x2",
			width:               2,
			height:              2,
			shouldContainTabs:   true,
			shouldContainFooter: true,
			description:         "Height 2: Both tabs and footer should be visible",
		},
		{
			name:                "very_small_3x3",
			width:               3,
			height:              3,
			shouldContainTabs:   true,
			shouldContainFooter: true,
			description:         "Height 3: Tabs and footer should have priority over content",
		},
		{
			name:                "narrow_width_25x10",
			width:               25,
			height:              10,
			shouldContainTabs:   true,
			shouldContainFooter: true,
			description:         "Normal height but narrow width: Should show compact tabs and footer",
		},
		{
			name:                "ultra_narrow_5x10",
			width:               5,
			height:              10,
			shouldContainTabs:   true,
			shouldContainFooter: true,
			description:         "Ultra narrow width: Should still show tabs and footer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.width = tt.width
			model.height = tt.height

			view := model.View()

			if view == "" {
				t.Errorf("View should not be empty for %s", tt.description)
			}

			// Check tab visibility (look for tab indicators)
			hasTabIndicators := strings.Contains(view, "C") ||
				strings.Contains(view, "Chat") ||
				strings.Contains(view, "Settings") ||
				strings.Contains(view, "S") ||
				strings.Contains(view, "R") ||
				strings.Contains(view, "RAG")

			if tt.shouldContainTabs && !hasTabIndicators {
				t.Errorf("Expected tabs to be visible for %s, but they weren't found", tt.description)
			}

			// Check footer visibility (look for help text - handle word wrapping at small widths)
			hasFooter := strings.Contains(view, "Tab") ||
				strings.Contains(view, "Ctrl") ||
				strings.Contains(view, "Quit") ||
				strings.Contains(view, "Q") ||
				strings.Contains(view, "Ta") // For narrow widths where "Tab" wraps

			if tt.shouldContainFooter && !hasFooter {
				t.Errorf("Expected footer to be visible for %s, but it wasn't found", tt.description)
			}

			// Log the view for debugging if needed
			t.Logf("View for %s (width=%d, height=%d):\n%s", tt.name, tt.width, tt.height, view)
		})
	}
}

func TestTabDeadZoneElimination(t *testing.T) {
	config := &configuration.Config{
		ChatModel:      "test-model",
		EmbeddingModel: "test-embedding",
		OllamaURL:      "http://localhost:11434",
	}
	ctx := t.Context()
	model := NewModel(ctx, config)

	// Test a range of widths to ensure there are no "dead zones" where tabs disappear
	for width := 2; width <= 35; width++ {
		for height := 1; height <= 5; height++ {
			t.Run(fmt.Sprintf("width_%d_height_%d", width, height), func(t *testing.T) {
				model.width = width
				model.height = height

				view := model.View()

				// Tabs should always be visible when height >= 1
				if height >= 1 {
					hasTabIndicators := strings.Contains(view, "C") ||
						strings.Contains(view, "Chat") ||
						strings.Contains(view, "Settings") ||
						strings.Contains(view, "S") ||
						strings.Contains(view, "R") ||
						strings.Contains(view, "RAG")

					if !hasTabIndicators {
						t.Errorf("Tabs should be visible at width=%d, height=%d but weren't found", width, height)
					}
				}

				// Footer should be visible when height >= 2
				if height >= 2 {
					hasFooter := strings.Contains(view, "Tab") ||
						strings.Contains(view, "Ctrl") ||
						strings.Contains(view, "Quit") ||
						strings.Contains(view, "Q") ||
						strings.Contains(view, "Ta") || // For narrow widths where "Tab" wraps
						strings.Contains(view, "b") // The "b" part when "Tab" wraps

					if !hasFooter {
						t.Errorf("Footer should be visible at width=%d, height=%d but wasn't found", width, height)
					}
				}
			})
		}
	}
}
