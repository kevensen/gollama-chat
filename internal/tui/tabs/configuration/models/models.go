package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// OllamaModel represents a model from the Ollama API
type OllamaModel struct {
	Name       string         `json:"name"`
	ModifiedAt string         `json:"modified_at"`
	Size       int64          `json:"size"`
	Digest     string         `json:"digest"`
	Details    map[string]any `json:"details"`
}

// OllamaModelsResponse represents the response from /api/tags
type OllamaModelsResponse struct {
	Models []OllamaModel `json:"models"`
}

// FetchModelsMsg represents the result of fetching models
type FetchModelsMsg struct {
	Models []OllamaModel
	Error  error
}

// SelectionMode represents whether we're selecting chat or embedding models
type SelectionMode int

const (
	ChatModelSelection SelectionMode = iota
	EmbeddingModelSelection
)

// Model represents the model selection panel
type Model struct {
	models         []OllamaModel
	filteredModels []OllamaModel
	cursor         int
	viewport       int
	height         int
	width          int
	loading        bool
	error          error
	mode           SelectionMode
	filter         string
	visible        bool
}

// NewModel creates a new model selection panel
func NewModel() Model {
	return Model{
		models:         []OllamaModel{},
		filteredModels: []OllamaModel{},
		cursor:         0,
		viewport:       0,
		height:         10,
		loading:        false,
		visible:        false,
	}
}

// Init initializes the model selection panel
func (m Model) Init() tea.Cmd {
	return nil
}

// SetVisible sets the visibility and mode of the model selection panel
func (m Model) SetVisible(visible bool, mode SelectionMode) Model {
	m.visible = visible
	m.mode = mode
	m.cursor = 0
	m.viewport = 0
	return m
}

// SetSize sets the dimensions of the model selection panel
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	return m
}

// FetchModels fetches available models from Ollama
func FetchModels(ollamaURL string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(ollamaURL + "/api/tags")
		if err != nil {
			return FetchModelsMsg{
				Models: nil,
				Error:  err,
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return FetchModelsMsg{
				Models: nil,
				Error:  fmt.Errorf("HTTP %d", resp.StatusCode),
			}
		}

		var response OllamaModelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return FetchModelsMsg{
				Models: nil,
				Error:  err,
			}
		}

		return FetchModelsMsg{
			Models: response.Models,
			Error:  nil,
		}
	})
}

// Update handles messages for the model selection panel
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case FetchModelsMsg:
		m.loading = false
		if msg.Error != nil {
			m.error = msg.Error
			m.models = []OllamaModel{}
		} else {
			m.error = nil
			m.models = msg.Models
		}
		m.filteredModels = m.filterModels(m.models)
		m.cursor = 0
		m.viewport = 0

	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.viewport {
					m.viewport = m.cursor
				}
			}

		case "down":
			if m.cursor < len(m.filteredModels)-1 {
				m.cursor++
				if m.cursor >= m.viewport+m.height-2 {
					m.viewport = m.cursor - m.height + 3
				}
			}

		case "enter":
			if len(m.filteredModels) > 0 {
				selectedModel := m.filteredModels[m.cursor]
				return m, tea.Cmd(func() tea.Msg {
					return ModelSelectedMsg{
						ModelName: selectedModel.Name,
						Mode:      m.mode,
					}
				})
			}
		}
	}

	return m, nil
}

// ModelSelectedMsg represents a model selection event
type ModelSelectedMsg struct {
	ModelName string
	Mode      SelectionMode
}

// isEmbeddingModel checks if a model is an embedding model based on name patterns
// This function uses common naming conventions for embedding models
func isEmbeddingModel(modelName, ollamaURL string) bool {
	// Quick name-based check using common embedding model patterns
	modelNameLower := strings.ToLower(modelName)

	// Common embedding model patterns
	embeddingPatterns := []string{
		"embed",      // Most common: nomic-embed-text, text-embedding-ada
		"embedding",  // embeddinggemma, sentence-embedding
		"nomic",      // nomic-embed-text models
		"bge",        // BGE (Beijing Academy of AI) embedding models
		"e5",         // E5 embedding models from Microsoft
		"sentence",   // sentence-transformers based models
		"mpnet",      // MPNet based embedding models
		"minilm",     // MiniLM based embedding models
		"distilbert", // DistilBERT based embedding models (when used for embeddings)
	}

	for _, pattern := range embeddingPatterns {
		if strings.Contains(modelNameLower, pattern) {
			// Additional check: avoid false positives for chat models that might contain these terms
			// For example, "embedded-llama" might be a chat model, not an embedding model
			if strings.Contains(modelNameLower, "chat") ||
				strings.Contains(modelNameLower, "instruct") ||
				strings.Contains(modelNameLower, "tool") {
				continue
			}
			return true
		}
	}

	// Future enhancement: Could check model capabilities via /api/show endpoint
	// This would be more accurate but slower due to additional API calls
	// TODO: Consider adding capability-based checking with caching if needed

	return false
}

// filterModels filters models based on the current mode and filter text
func (m Model) filterModels(models []OllamaModel) []OllamaModel {
	var filtered []OllamaModel

	for _, model := range models {
		// Apply text filter if any
		if m.filter != "" && !strings.Contains(strings.ToLower(model.Name), strings.ToLower(m.filter)) {
			continue
		}

		// Apply mode-specific filtering
		switch m.mode {
		case EmbeddingModelSelection:
			// Filter for embedding models based on model capabilities/patterns
			if isEmbeddingModel(model.Name, "") {
				filtered = append(filtered, model)
			}
		default:
			// Chat models - include all non-embedding models
			if !isEmbeddingModel(model.Name, "") {
				filtered = append(filtered, model)
			}
		}
	}

	return filtered
}

// View renders the model selection panel
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	var content []string

	// Title based on mode
	var title string
	switch m.mode {
	case ChatModelSelection:
		title = "Select Chat Model"
	case EmbeddingModelSelection:
		title = "Select Embedding Model"
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Align(lipgloss.Center)

	content = append(content, titleStyle.Render(title))
	content = append(content, "")

	// Handle different states
	if m.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Italic(true)
		content = append(content, loadingStyle.Render("⟳ Loading models..."))
	} else if m.error != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)
		content = append(content, errorStyle.Render("✗ Error: "+m.error.Error()))
		content = append(content, "")
		content = append(content, "Check Ollama connection and try again.")
	} else if len(m.filteredModels) == 0 {
		noModelsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
		content = append(content, noModelsStyle.Render("No models available"))
	} else {
		// Show models
		start := m.viewport
		end := min(start+m.height-2, len(m.filteredModels))

		for i := start; i < end; i++ {
			model := m.filteredModels[i]
			var style lipgloss.Style

			if i == m.cursor {
				// Selected model
				style = lipgloss.NewStyle().
					Foreground(lipgloss.Color("15")).
					Background(lipgloss.Color("62")).
					Bold(true)
			} else {
				// Regular model
				style = lipgloss.NewStyle().
					Foreground(lipgloss.Color("7"))
			}

			// Format model name and size
			sizeStr := formatSize(model.Size)
			modelLine := fmt.Sprintf("  %s (%s)", model.Name, sizeStr)
			content = append(content, style.Render(modelLine))
		}

		// Show scroll indicator if needed
		if len(m.filteredModels) > m.height-2 {
			scrollInfo := fmt.Sprintf("(%d/%d)", m.cursor+1, len(m.filteredModels))
			scrollStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Align(lipgloss.Right)
			content = append(content, "")
			content = append(content, scrollStyle.Render(scrollInfo))
		}
	}

	// Help text
	content = append(content, "")
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)
	content = append(content, helpStyle.Render("↑/↓: Navigate • Enter: Select • Esc: Cancel"))

	// Apply border and styling
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 1).
		Width(m.width - 2).
		Height(m.height)

	return panelStyle.Render(strings.Join(content, "\n"))
}

// formatSize formats a byte size into a human-readable string
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
