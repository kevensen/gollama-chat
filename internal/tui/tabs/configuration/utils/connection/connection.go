package connection

import (
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevensen/gollama-chat/internal/logging"
)

// Status represents the status of a server connection
type Status int

const (
	StatusUnknown Status = iota
	StatusConnected
	StatusDisconnected
	StatusChecking
)

// CheckMsg represents the result of a connection check
type CheckMsg struct {
	Server  string
	Status  Status
	Error   error
	FullURL string // The complete URL that was actually requested
}

// OllamaStatus checks if the Ollama server is reachable
func OllamaStatus(url string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		logger := logging.WithComponent("connection_check")
		fullURL := url + "/api/tags"
		logger.Info("Starting Ollama connection check", "url", url, "full_url", fullURL)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(fullURL)
		if err != nil {
			logger.Warn("Ollama connection failed", "url", url, "full_url", fullURL, "error", err)
			return CheckMsg{
				Server:  "ollama",
				Status:  StatusDisconnected,
				Error:   err,
				FullURL: fullURL,
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			logger.Info("Ollama connection successful", "url", url, "full_url", fullURL, "status_code", resp.StatusCode)
			return CheckMsg{
				Server:  "ollama",
				Status:  StatusConnected,
				Error:   nil,
				FullURL: fullURL,
			}
		}

		logger.Warn("Ollama connection returned non-OK status", "url", url, "full_url", fullURL, "status_code", resp.StatusCode)
		return CheckMsg{
			Server:  "ollama",
			Status:  StatusDisconnected,
			Error:   fmt.Errorf("HTTP %d", resp.StatusCode),
			FullURL: fullURL,
		}
	})
}

// ChromaDBStatus checks if the ChromaDB server is reachable
func ChromaDBStatus(url string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		logger := logging.WithComponent("connection_check")
		fullURL := url + "/api/v2/healthcheck"
		logger.Info("Starting ChromaDB connection check", "url", url, "full_url", fullURL)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(fullURL)
		if err != nil {
			logger.Warn("ChromaDB connection failed", "url", url, "full_url", fullURL, "error", err)
			return CheckMsg{
				Server:  "chromadb",
				Status:  StatusDisconnected,
				Error:   err,
				FullURL: fullURL,
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			logger.Info("ChromaDB connection successful", "url", url, "full_url", fullURL, "status_code", resp.StatusCode)
			return CheckMsg{
				Server:  "chromadb",
				Status:  StatusConnected,
				Error:   nil,
				FullURL: fullURL,
			}
		}

		logger.Warn("ChromaDB connection returned non-OK status", "url", url, "full_url", fullURL, "status_code", resp.StatusCode)
		return CheckMsg{
			Server:  "chromadb",
			Status:  StatusDisconnected,
			Error:   fmt.Errorf("HTTP %d", resp.StatusCode),
			FullURL: fullURL,
		}
	})
}
