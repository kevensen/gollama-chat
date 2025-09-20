package input

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// BenchmarkInsertCharacterDirect benchmarks the ultra-fast character insertion path
// This is the critical path for typing performance
func BenchmarkInsertCharacterDirect(b *testing.B) {
	tests := []struct {
		name       string
		setupText  string
		insertRune rune
		atPosition string // "start", "middle", "end"
	}{
		{"empty_input", "", 'a', "end"},
		{"append_to_short", "hello", 'a', "end"},
		{"append_to_medium", "hello world this is a medium length text", 'a', "end"},
		{"append_to_long", "this is a very long text that represents typical user input when writing detailed questions or descriptions that might span multiple lines worth of content", 'a', "end"},
		{"insert_at_start", "existing text", 'a', "start"},
		{"insert_at_middle", "existing text", 'a', "middle"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				model := NewModel()
				model.SetValue(tt.setupText)

				// Position cursor based on test case
				switch tt.atPosition {
				case "start":
					model.cursor = 0
				case "middle":
					model.cursor = len(tt.setupText) / 2
				case "end":
					model.cursor = len(tt.setupText)
				}

				// Benchmark the insertion
				model.InsertCharacterDirect(tt.insertRune)
			}
		})
	}
}

// BenchmarkTypingSequence benchmarks a realistic typing sequence
// This simulates actual user typing behavior
func BenchmarkTypingSequence(b *testing.B) {
	sequences := []struct {
		name string
		text string
	}{
		{"short_question", "What is Go?"},
		{"medium_question", "How do I implement error handling in Go applications?"},
		{"long_question", "Can you explain the differences between channels and mutexes for concurrent programming in Go, and when should I use each approach?"},
		{"very_long_question", "I'm working on a large Go application with multiple microservices and I need to implement proper observability including logging, metrics, and tracing. What are the best practices for setting up a comprehensive monitoring solution?"},
	}

	for _, seq := range sequences {
		b.Run(seq.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				model := NewModel()

				// Simulate typing each character
				for _, char := range seq.text {
					model.InsertCharacterDirect(char)
				}
			}
		})
	}
}

// BenchmarkBackspaceOperations benchmarks backspace performance
func BenchmarkBackspaceOperations(b *testing.B) {
	tests := []struct {
		name     string
		text     string
		delCount int
	}{
		{"delete_from_short", "hello", 2},
		{"delete_from_medium", "hello world test", 5},
		{"delete_from_long", "this is a very long text that we will delete from", 10},
		{"delete_all_short", "hello", 5},
		{"delete_all_medium", "hello world test", 17},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				model := NewModel()
				model.SetValue(tt.text)

				// Delete the specified number of characters
				for j := 0; j < tt.delCount && model.cursor > 0; j++ {
					model.Backspace()
				}
			}
		})
	}
}

// BenchmarkCursorMovement benchmarks cursor navigation performance
func BenchmarkCursorMovement(b *testing.B) {
	text := "this is a sample text for cursor movement testing with reasonable length"

	tests := []struct {
		name      string
		operation func(*Model)
	}{
		{"move_left", func(m *Model) { m.MoveCursorLeft() }},
		{"move_right", func(m *Model) { m.MoveCursorRight() }},
		{"move_to_start", func(m *Model) { m.cursor = 0 }},
		{"move_to_end", func(m *Model) { m.cursor = len(m.value) }},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				model := NewModel()
				model.SetValue(text)
				model.cursor = len(text) / 2 // Start in middle

				tt.operation(&model)
			}
		})
	}
}

// BenchmarkUpdate benchmarks the Update method with various message types
func BenchmarkUpdate(b *testing.B) {
	tests := []struct {
		name string
		msg  tea.Msg
	}{
		{"key_rune_ascii", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}},
		{"key_backspace", tea.KeyMsg{Type: tea.KeyBackspace}},
		{"key_left", tea.KeyMsg{Type: tea.KeyLeft}},
		{"key_right", tea.KeyMsg{Type: tea.KeyRight}},
		{"window_size", tea.WindowSizeMsg{Width: 100, Height: 3}},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			model := NewModel()
			model.SetValue("sample text for testing")

			for i := 0; i < b.N; i++ {
				model.Update(tt.msg)
			}
		})
	}
}

// BenchmarkView benchmarks the rendering performance
func BenchmarkView(b *testing.B) {
	tests := []struct {
		name   string
		text   string
		cursor int
		setup  func(*Model)
	}{
		{"empty_input", "", 0, nil},
		{"short_text_end", "hello", 5, nil},
		{"short_text_middle", "hello", 2, nil},
		{"medium_text_end", "hello world this is medium text", 31, nil},
		{"medium_text_middle", "hello world this is medium text", 16, nil},
		{"long_text_end", "this is a very long text that might cause performance issues if rendering is not optimized properly for the view method", 114, nil},
		{"long_text_middle", "this is a very long text that might cause performance issues if rendering is not optimized properly for the view method", 57, nil},
		{"loading_state", "hello", 5, func(m *Model) {
			m.SetLoading(true)
			m.SetRAGStatus("Searching documents...")
		}},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			model := NewModel()
			model.SetValue(tt.text)
			model.cursor = tt.cursor

			if tt.setup != nil {
				tt.setup(&model)
			}

			for i := 0; i < b.N; i++ {
				_ = model.View()
			}
		})
	}
}

// BenchmarkRealWorldTyping simulates realistic typing patterns with mixed operations
func BenchmarkRealWorldTyping(b *testing.B) {
	scenarios := []struct {
		name   string
		script func(*Model)
	}{
		{
			"type_and_correct",
			func(m *Model) {
				// Type "Hello wrold"
				for _, char := range "Hello wrold" {
					m.InsertCharacterDirect(char)
				}
				// Go back and fix "wrold" -> "world"
				m.MoveCursorLeft() // d
				m.MoveCursorLeft() // l
				m.MoveCursorLeft() // r
				m.MoveCursorLeft() // o
				m.Backspace()      // delete 'r'
				m.Backspace()      // delete 'o'
				for _, char := range "or" {
					m.InsertCharacterDirect(char)
				}
			},
		},
		{
			"type_long_with_corrections",
			func(m *Model) {
				// Type a longer text with some corrections
				text := "Can you help me understand how to implement a REST API"
				for _, char := range text {
					m.InsertCharacterDirect(char)
				}
				// Add " in Go?"
				for _, char := range " in Go?" {
					m.InsertCharacterDirect(char)
				}
				// Go back and insert "efficiently" before "in Go"
				for i := 0; i < 7; i++ { // Move back past " in Go?"
					m.MoveCursorLeft()
				}
				for _, char := range "efficiently " {
					m.InsertCharacterDirect(char)
				}
			},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				model := NewModel()
				scenario.script(&model)
			}
		})
	}
}

// BenchmarkUnicodeHandling benchmarks performance with Unicode characters
func BenchmarkUnicodeHandling(b *testing.B) {
	tests := []struct {
		name  string
		chars []rune
	}{
		{"ascii_only", []rune("hello world")},
		{"mixed_unicode", []rune("Hello 世界 🌍")},
		{"emoji_heavy", []rune("🚀 Go is awesome! 💯 🎉")},
		{"accented_chars", []rune("café résumé naïve")},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				model := NewModel()
				for _, char := range tt.chars {
					model.InsertCharacterDirect(char)
				}
			}
		})
	}
}

// BenchmarkMemoryAllocation focuses on allocation-heavy operations
func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("repeated_insertions", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			model := NewModel()
			// Insert 100 characters to test allocation behavior
			for j := 0; j < 100; j++ {
				model.InsertCharacterDirect('a')
			}
		}
	})

	b.Run("resize_operations", func(b *testing.B) {
		model := NewModel()
		for i := 0; i < b.N; i++ {
			// Simulate window resizing
			model.SetSize(80+i%50, 3) // Vary width from 80 to 130
		}
	})
}
