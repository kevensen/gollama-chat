package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevensen/gollama-chat/internal/configuration"
	"github.com/kevensen/gollama-chat/internal/tui/tabs/chat"
)

// createBenchmarkConfig creates a standardized config for benchmarking
func createBenchmarkConfig() *configuration.Config {
	return &configuration.Config{
		OllamaURL:           "http://localhost:11434",
		ChatModel:           "llama3.2:3b",
		EmbeddingModel:      "embeddinggemma:latest",
		RAGEnabled:          true,
		ChromaDBURL:         "http://localhost:8000",
		ChromaDBDistance:    1.0,
		MaxDocuments:        5,
		DarkMode:            false,
		SelectedCollections: make(map[string]bool),
		DefaultSystemPrompt: "You are a helpful assistant.",
	}
}

// BenchmarkFullInputPipeline benchmarks the complete input path from TUI.Update to input component
// This tests the entire flow: TUI.Update -> Chat.Update -> Input.InsertCharacterDirect
func BenchmarkFullInputPipeline(b *testing.B) {
	tests := []struct {
		name        string
		setupText   string
		inputChar   rune
		description string
	}{
		{"empty_input_ascii", "", 'a', "Single ASCII character on empty input"},
		{"short_text_append", "hello", 'a', "Append to short text"},
		{"medium_text_append", "hello world this is medium length text", 'a', "Append to medium text"},
		{"long_text_append", "this is a very long text that represents typical user input when writing detailed questions or descriptions that might span multiple lines worth of content", 'a', "Append to long text"},
		{"space_character", "hello", ' ', "Space character handling"},
		{"common_chars", "test", 'e', "Common character insertion"},
		{"punctuation", "hello world", '.', "Punctuation insertion"},
		{"numbers", "value: ", '4', "Number insertion"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Setup TUI model with realistic configuration
			config := createBenchmarkConfig()
			ctx := context.Background()
			model := NewModel(ctx, config)

			// Initialize with realistic window size
			model.width = 100
			model.height = 30
			model.activeTab = ChatTab

			// Setup initial text in input if needed
			if tt.setupText != "" {
				// Type the setup text first
				for _, char := range tt.setupText {
					var keyMsg tea.Msg
					if char == ' ' {
						keyMsg = tea.KeyMsg{Type: tea.KeySpace}
					} else {
						keyMsg = tea.KeyMsg{
							Type:  tea.KeyRunes,
							Runes: []rune{char},
						}
					}
					model.Update(keyMsg)
				}
			}

			// Create the key message for the character we want to benchmark
			var keyMsg tea.Msg
			if tt.inputChar == ' ' {
				keyMsg = tea.KeyMsg{Type: tea.KeySpace}
			} else {
				keyMsg = tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune{tt.inputChar},
				}
			}

			// Benchmark the full pipeline
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// This tests the complete path:
				// TUI.Update -> Chat.Update -> Input.InsertCharacterDirect
				model.Update(keyMsg)
			}
		})
	}
}

// BenchmarkWithoutUltraFastPath tests performance if ultra-fast path is removed
// This simulates removing the TUI-level fast path and only using the chat-level fast path
func BenchmarkWithoutUltraFastPath(b *testing.B) {
	config := createBenchmarkConfig()
	ctx := context.Background()

	b.Run("ascii_without_ultra_fast_path", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'a'},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate what happens if ultra-fast path is removed:
			// Force message to go through normal routing to chat model
			chatModel, chatCmd := model.chatModel.Update(keyMsg)
			model.chatModel = chatModel.(chat.Model)
			_ = chatCmd // Ignore command for benchmark
		}
	})

	b.Run("space_without_ultra_fast_path", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		keyMsg := tea.KeyMsg{Type: tea.KeySpace}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate normal path routing for space character
			chatModel, chatCmd := model.chatModel.Update(keyMsg)
			model.chatModel = chatModel.(chat.Model)
			_ = chatCmd
		}
	})

	b.Run("comparison_with_ultra_fast_path", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'a'},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Use the current ultra-fast path
			model.Update(keyMsg)
		}
	})
}

// BenchmarkFastPathEfficiency compares fast path vs normal path performance
func BenchmarkFastPathEfficiency(b *testing.B) {
	config := createBenchmarkConfig()
	ctx := context.Background()

	b.Run("fast_path_ascii", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'a'},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			model.Update(keyMsg)
		}
	})

	b.Run("normal_path_non_ascii", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		// Use a non-ASCII character to force normal path
		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'é'},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			model.Update(keyMsg)
		}
	})

	b.Run("control_keys", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		keyMsg := tea.KeyMsg{
			Type: tea.KeyBackspace,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			model.Update(keyMsg)
		}
	})
}

// BenchmarkTabSwitchingImpact tests if tab switching affects input performance
func BenchmarkTabSwitchingImpact(b *testing.B) {
	config := createBenchmarkConfig()
	ctx := context.Background()

	b.Run("chat_tab_active", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'a'},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			model.Update(keyMsg)
		}
	})

	b.Run("config_tab_active", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ConfigTab

		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'a'},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			model.Update(keyMsg)
		}
	})
}

// BenchmarkRealisticTypingWorkload simulates real-world typing patterns through the full pipeline
func BenchmarkRealisticTypingWorkload(b *testing.B) {
	scenarios := []struct {
		name        string
		description string
		sequence    []rune
	}{
		{
			"short_question",
			"Typing a short question",
			[]rune("What is Go?"),
		},
		{
			"medium_question",
			"Typing a medium-length question",
			[]rune("How do I implement error handling in Go applications?"),
		},
		{
			"long_question_with_spaces",
			"Typing a long question with many spaces",
			[]rune("Can you explain the differences between channels and mutexes for concurrent programming in Go, and when should I use each approach?"),
		},
		{
			"programming_snippet",
			"Typing code-like content",
			[]rune("func main() { fmt.Println(\"Hello World\") }"),
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := createBenchmarkConfig()
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				model := NewModel(ctx, config)
				model.width = 100
				model.height = 30
				model.activeTab = ChatTab

				// Type the entire sequence
				for _, char := range scenario.sequence {
					var keyMsg tea.Msg
					if char == ' ' {
						keyMsg = tea.KeyMsg{Type: tea.KeySpace}
					} else {
						keyMsg = tea.KeyMsg{
							Type:  tea.KeyRunes,
							Runes: []rune{char},
						}
					}
					model.Update(keyMsg)
				}
			}
		})
	}
}

// BenchmarkWindowSizeUpdates tests performance impact of window resize events during typing
func BenchmarkWindowSizeUpdates(b *testing.B) {
	config := createBenchmarkConfig()
	ctx := context.Background()

	b.Run("typing_with_stable_window", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'a'},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			model.Update(keyMsg)
		}
	})

	b.Run("typing_with_window_resize", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'a'},
		}

		resizeMsg := tea.WindowSizeMsg{
			Width:  120,
			Height: 40,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Alternate between typing and resizing to simulate real conditions
			if i%2 == 0 {
				model.Update(keyMsg)
			} else {
				model.Update(resizeMsg)
			}
		}
	})
}

// BenchmarkMemoryAllocationFullPipeline tests memory allocation patterns in the full pipeline
func BenchmarkMemoryAllocationFullPipeline(b *testing.B) {
	config := createBenchmarkConfig()
	ctx := context.Background()

	b.Run("single_character_allocation", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'a'},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			model.Update(keyMsg)
		}
	})

	b.Run("batch_character_allocation", func(b *testing.B) {
		model := NewModel(ctx, config)
		model.width = 100
		model.height = 30
		model.activeTab = ChatTab

		chars := []rune("hello world")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Type multiple characters to test allocation patterns
			for _, char := range chars {
				keyMsg := tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune{char},
				}
				model.Update(keyMsg)
			}
		}
	})
}

// BenchmarkTypingWithoutUltraFastPath tests realistic typing scenarios without ultra-fast path
func BenchmarkTypingWithoutUltraFastPath(b *testing.B) {
	scenarios := []struct {
		name     string
		sequence []rune
	}{
		{"short_question", []rune("What is Go?")},
		{"medium_question", []rune("How do I implement error handling?")},
		{"programming_text", []rune("func main() { fmt.Println() }")},
	}

	for _, scenario := range scenarios {
		b.Run("without_ultra_fast_"+scenario.name, func(b *testing.B) {
			config := createBenchmarkConfig()
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				model := NewModel(ctx, config)
				model.width = 100
				model.height = 30
				model.activeTab = ChatTab

				// Type each character using only the chat model (no ultra-fast path)
				for _, char := range scenario.sequence {
					var keyMsg tea.Msg
					if char == ' ' {
						keyMsg = tea.KeyMsg{Type: tea.KeySpace}
					} else {
						keyMsg = tea.KeyMsg{
							Type:  tea.KeyRunes,
							Runes: []rune{char},
						}
					}
					// Route directly to chat model (simulating removed ultra-fast path)
					chatModel, _ := model.chatModel.Update(keyMsg)
					model.chatModel = chatModel.(chat.Model)
				}
			}
		})

		b.Run("with_ultra_fast_"+scenario.name, func(b *testing.B) {
			config := createBenchmarkConfig()
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				model := NewModel(ctx, config)
				model.width = 100
				model.height = 30
				model.activeTab = ChatTab

				// Type each character using the current ultra-fast path
				for _, char := range scenario.sequence {
					var keyMsg tea.Msg
					if char == ' ' {
						keyMsg = tea.KeyMsg{Type: tea.KeySpace}
					} else {
						keyMsg = tea.KeyMsg{
							Type:  tea.KeyRunes,
							Runes: []rune{char},
						}
					}
					model.Update(keyMsg)
				}
			}
		})
	}
}
