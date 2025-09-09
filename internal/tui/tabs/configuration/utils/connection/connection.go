package connection

import (
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url + "/api/tags")
		if err != nil {
			return CheckMsg{
				Server: "ollama",
				Status: StatusDisconnected,
				Error:  err,
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return CheckMsg{
				Server: "ollama",
				Status: StatusConnected,
				Error:  nil,
			}
		}

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
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url + "/api/v2")
		if err != nil {
			return CheckMsg{
				Server: "chromadb",
				Status: StatusDisconnected,
				Error:  err,
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return CheckMsg{
				Server: "chromadb",
				Status: StatusConnected,
				Error:  nil,
			}
		}

		return CheckMsg{
			Server: "chromadb",
			Status: StatusDisconnected,
			Error:  fmt.Errorf("HTTP %d", resp.StatusCode),
		}
	})
}
