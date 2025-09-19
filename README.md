# gollama-chat

A text user interface (TUI) for chatting with Large Language Models via Ollama.

![Chat Interface](screenshot.png)

## Features

- **Clean TUI**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for a responsive terminal interface
- **Tab-based Navigation**: Switch between Chat and Settings tabs
- **Ollama Integration**: Chat with any Ollama-supported model
- **Configurable**: Customize Ollama URL, model, temperature, and more
- **Message History**: In-memory chat history (cleared on exit)
- **Keyboard Navigation**: Fully keyboard-driven interface

## Installation

### Prerequisites

- Go 1.21 or later
- [Ollama](https://ollama.ai/) installed and running

### Building from Source

```bash
git clone https://github.com/kevensen/gollama-chat.git
cd gollama-chat
make build
```

## Usage

### Starting the Application

```bash
# Run directly
make run

# Or run the binary
./bin/gollama-chat
```

### Configuration

The application stores its configuration in:
- **Linux/macOS**: `~/.config/gollama/settings.json`
- **Windows**: `%APPDATA%\gollama\settings.json`

Default configuration:
```json
{
  "ollama_url": "http://localhost:11434",
  "default_model": "llama3.2",
  "max_tokens": 2048,
  "temperature": 0.7
}
```

### Keyboard Shortcuts

#### Global
- `Tab` / `Shift+Tab` - Switch between tabs
- `Ctrl+C` / `q` - Quit application

#### Chat Tab
- `Enter` - Send message
- `Ctrl+L` - Clear chat history
- `Ctrl+S` - Toggle system prompt display
- `↑` / `↓` - Scroll through messages
- `←` / `→` - Move cursor in input

#### Settings Tab
- `↑` / `↓` - Navigate between fields
- `Enter` - Edit selected field
- `S` - Save configuration
- `R` - Reset to defaults
- `Esc` - Cancel editing

## Configuration Options

| Setting | Description | Default |
|---------|-------------|---------|
| `ollama_url` | URL of the Ollama server | `http://localhost:11434` |
| `default_model` | Model to use for chat | `llama3.2` |
| `max_tokens` | Maximum tokens for responses | `2048` |
| `temperature` | Creativity level (0.0-2.0) | `0.7` |

## Project Structure

```
gollama-chat/
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── configuration/          # Configuration management
│   │   └── configuration.go
│   └── tui/                    # Text User Interface
│       ├── tui/
│       │   └── tui.go         # Main TUI controller
│       └── tabs/              # Tab implementations
│           ├── chat/
│           │   └── chat.go    # Chat functionality
│           └── configuration/
│               └── configuration.go # Settings tab
├── bin/                       # Built binaries
├── Makefile                   # Build automation
├── go.mod                     # Go module definition
└── README.md                  # This file
```

## Development

### Building

```bash
make build
```

### Running

```bash
make run
```

### Development Workflow

```bash
make dev  # Runs fmt, vet, test, and build
```

### Testing

```bash
make test
```

## Architecture

The application follows principles of high cohesion and loose coupling:

- **Configuration**: Centralized configuration management with JSON persistence
- **TUI Controller**: Main application state and tab management
- **Composable Views**: Each tab is a separate, reusable component
- **Message Passing**: Uses Bubble Tea's message-based architecture

## Requirements

- Ollama server running locally or accessible via network
- Terminal with color support
- Go 1.21+ for building from source

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make dev` to ensure code quality
5. Submit a pull request

## Roadmap

- [ ] Message persistence options
- [ ] Multiple conversation support
- [ ] Model switching during chat
- [ ] Export conversations
- [ ] ChromaDB integration for RAG
- [ ] Streaming responses
- [ ] Custom themes
