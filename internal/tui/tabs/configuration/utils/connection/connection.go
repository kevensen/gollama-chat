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
	Server string
	Status Status
	Error  error
}

// OllamaStatus checks if the Ollama server is reachable
func OllamaStatus(url string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		logger := logging.WithComponent("connection_check")
		logger.Info("Starting Ollama connection check", "url", url)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url + "/api/tags")
		if err != nil {
			logger.Warn("Ollama connection failed", "url", url, "error", err)
			return CheckMsg{
				Server: "ollama",
				Status: StatusDisconnected,
				Error:  err,
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			logger.Info("Ollama connection successful", "url", url, "status_code", resp.StatusCode)
			return CheckMsg{
				Server: "ollama",
				Status: StatusConnected,
				Error:  nil,
			}
		}

		logger.Warn("Ollama connection returned non-OK status", "url", url, "status_code", resp.StatusCode)
		return CheckMsg{
			Server: "ollama",
			Status: StatusDisconnected,
			Error:  fmt.Errorf("HTTP %d", resp.StatusCode),
		}
	})
}

// ChromaDBStatus checks if the ChromaDB server is reachable
func ChromaDBStatus(url string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		logger := logging.WithComponent("connection_check")
		logger.Info("Starting ChromaDB connection check", "url", url)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url + "/api/v1")
		if err != nil {
			logger.Warn("ChromaDB connection failed", "url", url, "error", err)
			return CheckMsg{
				Server: "chromadb",
				Status: StatusDisconnected,
				Error:  err,
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			logger.Info("ChromaDB connection successful", "url", url, "status_code", resp.StatusCode)
			return CheckMsg{
				Server: "chromadb",
				Status: StatusConnected,
				Error:  nil,
			}
		}

		logger.Warn("ChromaDB connection returned non-OK status", "url", url, "status_code", resp.StatusCode)
		return CheckMsg{
			Server: "chromadb",
			Status: StatusDisconnected,
			Error:  fmt.Errorf("HTTP %d", resp.StatusCode),
		}
	})
}
