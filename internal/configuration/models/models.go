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
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// OllamaModelsResponse represents the response from /api/tags
type OllamaModelsResponse struct {
	Models []OllamaModel `json:"models"`
}

// LoadStatus represents the status of loading models
type LoadStatus int

const (
	LoadStatusIdle LoadStatus = iota
	LoadStatusLoading
	LoadStatusLoaded
	LoadStatusError
)

// ModelsLoadedMsg represents the result of loading models
type ModelsLoadedMsg struct {
	Models []OllamaModel
	Error  error
}

// ModelSelectedMsg represents a model selection
type ModelSelectedMsg struct {
	ModelName string
}

// Model represents the model selection panel
type Model struct {
	ollamaURL     string
	models        []OllamaModel
	selectedIndex int
	loadStatus    LoadStatus
	errorMessage  string
	width         int
	height        int
	visible       bool
	cursor        int
	scrollOffset  int
}

// NewModel creates a new model selection model
func NewModel(ollamaURL string) Model {
	return Model{
		ollamaURL:     ollamaURL,
		models:        []OllamaModel{},
		selectedIndex: 0,
		loadStatus:    LoadStatusIdle,
		width:         40,
		height:        15,
		visible:       false,
		cursor:        0,
		scrollOffset:  0,
	}
}

// SetVisible sets the visibility of the model panel
func (m Model) SetVisible(visible bool) Model {
	m.visible = visible
	if visible && m.loadStatus == LoadStatusIdle {
		m.loadStatus = LoadStatusLoading
	}
	return m
}

// SetSize sets the dimensions of the model panel
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	return m
}

// SetOllamaURL updates the Ollama URL and resets the model state
func (m Model) SetOllamaURL(url string) Model {
	m.ollamaURL = url
	m.loadStatus = LoadStatusIdle
	m.models = []OllamaModel{}
	m.errorMessage = ""
	m.selectedIndex = 0
	m.cursor = 0
	m.scrollOffset = 0
	return m
}

// Init initializes the model selection panel
func (m Model) Init() tea.Cmd {
	if m.visible && m.loadStatus == LoadStatusLoading {
		return m.loadModels()
	}
	return nil
}

// Update handles messages and updates the model selection panel
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ModelsLoadedMsg:
		if msg.Error != nil {
			m.loadStatus = LoadStatusError
			m.errorMessage = fmt.Sprintf("Failed to load models: %s", msg.Error.Error())
		} else {
			m.loadStatus = LoadStatusLoaded
			m.models = msg.Models
			m.errorMessage = ""
		}

	case tea.KeyMsg:
		if !m.visible {
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.scrollOffset {
					m.scrollOffset = m.cursor
				}
			}

		case "down", "j":
			if m.cursor < len(m.models)-1 {
				m.cursor++
				maxVisibleItems := m.height - 4 // Account for border and title
				if m.cursor >= m.scrollOffset+maxVisibleItems {
					m.scrollOffset = m.cursor - maxVisibleItems + 1
				}
			}

		case "enter":
			if m.loadStatus == LoadStatusLoaded && len(m.models) > 0 && m.cursor < len(m.models) {
				selectedModel := m.models[m.cursor]
				return m, func() tea.Msg {
					return ModelSelectedMsg{ModelName: selectedModel.Name}
				}
			}

		case "r":
			// Refresh models
			m.loadStatus = LoadStatusLoading
			m.errorMessage = ""
			return m, m.loadModels()
		}
	}

	return m, nil
}

// View renders the model selection panel
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	var content []string

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Align(lipgloss.Center)

	content = append(content, titleStyle.Render("Available Models"))

	// Content based on load status
	switch m.loadStatus {
	case LoadStatusLoading:
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Italic(true)
		content = append(content, "", loadingStyle.Render("Loading models..."))

	case LoadStatusError:
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)
		content = append(content, "", errorStyle.Render(m.errorMessage))
		content = append(content, "")
		helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		content = append(content, helpStyle.Render("Press 'r' to retry"))

	case LoadStatusLoaded:
		if len(m.models) == 0 {
			noModelsStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true)
			content = append(content, "", noModelsStyle.Render("No models available"))
		} else {
			content = append(content, "")
			content = append(content, m.renderModelList()...)
			content = append(content, "")
			helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			content = append(content, helpStyle.Render("↑/↓: Navigate • Enter: Select • r: Refresh"))
		}

	default:
		// Idle state
		idleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
		content = append(content, "", idleStyle.Render("Select a model field to view models"))
	}

	// Join content and apply border
	contentStr := strings.Join(content, "\n")

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Width(m.width).
		Height(m.height)

	return panelStyle.Render(contentStr)
}

// renderModelList renders the scrollable list of models
func (m Model) renderModelList() []string {
	var lines []string
	maxVisibleItems := m.height - 4 // Account for border, title, and help text

	start := m.scrollOffset
	end := start + maxVisibleItems
	if end > len(m.models) {
		end = len(m.models)
	}

	for i := start; i < end; i++ {
		model := m.models[i]
		line := m.formatModelLine(model, i == m.cursor)
		lines = append(lines, line)
	}

	// Add scroll indicators
	if m.scrollOffset > 0 {
		scrollUpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		lines = append([]string{scrollUpStyle.Render("↑ More above")}, lines...)
	}

	if end < len(m.models) {
		scrollDownStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		lines = append(lines, scrollDownStyle.Render("↓ More below"))
	}

	return lines
}

// formatModelLine formats a single model line
func (m Model) formatModelLine(model OllamaModel, isSelected bool) string {
	var modelStyle lipgloss.Style

	if isSelected {
		modelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Bold(true)
	} else {
		modelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))
	}

	// Format the model name, truncate if too long
	maxWidth := m.width - 6 // Account for padding and potential selection indicator
	modelName := model.Name
	if len(modelName) > maxWidth {
		modelName = modelName[:maxWidth-3] + "..."
	}

	prefix := "  "
	if isSelected {
		prefix = "▶ "
	}

	return modelStyle.Render(prefix + modelName)
}

// loadModels loads models from the Ollama API
func (m Model) loadModels() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(m.ollamaURL + "/api/tags")
		if err != nil {
			return ModelsLoadedMsg{
				Models: nil,
				Error:  err,
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ModelsLoadedMsg{
				Models: nil,
				Error:  fmt.Errorf("HTTP %d", resp.StatusCode),
			}
		}

		var response OllamaModelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ModelsLoadedMsg{
				Models: nil,
				Error:  fmt.Errorf("failed to decode response: %w", err),
			}
		}

		return ModelsLoadedMsg{
			Models: response.Models,
			Error:  nil,
		}
	})
}

// IsVisible returns whether the panel is visible
func (m Model) IsVisible() bool {
	return m.visible
}

// GetSelectedModel returns the currently selected model name
func (m Model) GetSelectedModel() string {
	if m.loadStatus == LoadStatusLoaded && len(m.models) > 0 && m.cursor < len(m.models) {
		return m.models[m.cursor].Name
	}
	return ""
}
