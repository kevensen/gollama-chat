package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/kevensen/gollama-chat/internal/configuration"
)

type ResizeMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin for development
	},
}

type WebServer struct {
	port   int
	config *configuration.Config
}

func New(port int, config *configuration.Config) *WebServer {
	return &WebServer{
		port:   port,
		config: config,
	}
}

func (ws *WebServer) Start(ctx context.Context) error {
	http.HandleFunc("/", ws.handleHome)
	http.HandleFunc("/ws", ws.handleWebSocket)

	addr := ":" + strconv.Itoa(ws.port)
	fmt.Printf("Starting gollama-chat web server on port %d...\n", ws.port)
	fmt.Printf("Open your browser to http://localhost:%d\n", ws.port)

	return http.ListenAndServe(addr, nil)
}

func (ws *WebServer) handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Gollama Chat</title>
    <meta charset="utf-8">
    <style>
        body {
            margin: 0;
            padding: 0;
            background: #000;
            font-family: 'Courier New', monospace;
        }
        #terminal {
            width: 100vw;
            height: 100vh;
            background: #000;
        }
    </style>
    <script src="https://unpkg.com/xterm@5.3.0/lib/xterm.js"></script>
    <script src="https://unpkg.com/xterm-addon-fit@0.8.0/lib/xterm-addon-fit.js"></script>
    <link rel="stylesheet" href="https://unpkg.com/xterm@5.3.0/css/xterm.css" />
</head>
<body>
    <div id="terminal"></div>
    <script>
        const term = new Terminal({
            cursorBlink: true,
            theme: {
                background: '#000000',
                foreground: '#ffffff'
            }
        });
        
        const fitAddon = new FitAddon.FitAddon();
        term.loadAddon(fitAddon);
        
        term.open(document.getElementById('terminal'));
        fitAddon.fit();
        
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(protocol + '//' + window.location.host + '/ws');
        
        ws.onopen = function() {
            term.writeln('Connected to gollama-chat...\r\n');
        };
        
        ws.onmessage = function(event) {
            if (event.data instanceof Blob) {
                // Handle binary data with improved decoding
                event.data.arrayBuffer().then(function(buffer) {
                    const uint8Array = new Uint8Array(buffer);
                    // Use latin1 decoding to preserve all bytes, then convert
                    let text = '';
                    for (let i = 0; i < uint8Array.length; i++) {
                        text += String.fromCharCode(uint8Array[i]);
                    }
                    term.write(text);
                });
            } else {
                // Handle text data
                term.write(event.data);
            }
        };
        
        ws.onclose = function() {
            term.writeln('\r\n\r\nConnection closed.');
        };
        
        ws.onerror = function(error) {
            term.writeln('WebSocket error: ' + error);
        };
        
        term.onData(function(data) {
            ws.send(data);
        });
        
        window.addEventListener('resize', function() {
            fitAddon.fit();
            ws.send(JSON.stringify({
                type: 'resize',
                cols: term.cols,
                rows: term.rows
            }));
        });
        
        // Initial resize
        setTimeout(function() {
            fitAddon.fit();
            ws.send(JSON.stringify({
                type: 'resize',
                cols: term.cols,
                rows: term.rows
            }));
        }, 100);
    </script>
</body>
</html>`

	t, _ := template.New("index").Parse(tmpl)
	t.Execute(w, nil)
}

func (ws *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Start the TUI application in a PTY
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Failed to get executable path: %v", err)
		return
	}

	cmd := exec.Command(execPath, "-child")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("Failed to start PTY: %v", err)
		return
	}
	defer ptmx.Close()

	// Set initial PTY size to a reasonable default
	pty.Setsize(ptmx, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	})

	var wg sync.WaitGroup
	wg.Add(2)

	// PTY to WebSocket
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("PTY read error: %v", err)
				}
				return
			}
			// Use binary message to handle ANSI escape sequences and non-UTF-8 data
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}()

	// WebSocket to PTY
	go func() {
		defer wg.Done()
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WebSocket read error: %v", err)
				return
			}

			if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
				// Check if it's a resize message (only for text messages)
				if messageType == websocket.TextMessage && len(data) > 0 && data[0] == '{' {
					var resizeMsg ResizeMessage
					if err := json.Unmarshal(data, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
						// Handle resize
						if err := pty.Setsize(ptmx, &pty.Winsize{
							Rows: uint16(resizeMsg.Rows),
							Cols: uint16(resizeMsg.Cols),
						}); err != nil {
							log.Printf("Failed to resize PTY: %v", err)
						}
						continue
					}
				}

				if _, err := ptmx.Write(data); err != nil {
					log.Printf("PTY write error: %v", err)
					return
				}
			}
		}
	}()

	wg.Wait()
}
