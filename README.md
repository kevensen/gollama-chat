# gollama-chat

<div align="center">
  <img src="images/Icon.png" alt="Icon" width="200"/>
</div>

A text user interface (TUI) for chatting with Large Language Models via Ollama.

<div align="center">
  <img src="images/Screenshot.png" alt="Icon" width="800"/>
</div>


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

#### Terminal Mode (Default)
```bash
# Run directly
make run

# Or run the binary
./bin/gollama-chat
```

#### Download the Binary

1. Go to the releases page: https://github.com/kevensen/gollama-chat/releases.
2. Download your binary to a directory in yourpath.
3. Make the binary executable
```
chmod u+x <path to binary>
```
4. If on Macos, you will need ensure the binary is trusted:
```
xattr -d com.apple.quarantine <path to binary>
```

### Command Line Options

```bash
gollama-chat [options]

Options:
  -h                Show help
```

Examples:
```bash
# Run in terminal mode
./gollama-chat
```

### Configuration

The application stores its configuration in:
- **Linux/macOS**: `~/.local/share/gollama-chat/settings/settings.json`
- **Windows**: `%LOCALAPPDATA%\gollama-chat\settings\settings.json`

Default configuration:
```json
{
  "chatModel": "llama3.3:latest",
  "embeddingModel": "embeddinggemma:latest",
  "ragEnabled": true,
  "ollamaURL": "http://localhost:11434",
  "chromaDBURL": "http://localhost:8000",
  "chromaDBDistance": 1.0,
  "maxDocuments": 5,
  "selectedCollections": {},
  "defaultSystemPrompt": "You are a helpful Q&A bot. Your purpose is to provide direct, accurate answers to user questions. When providing lists of items (such as countries, capitals, features, etc.), format your response using proper numbered or bulleted lists. Be consistent in your formatting. If you don't know the answer, state that you are unable to provide a response."
}
```

### Keyboard Shortcuts

#### Global
- `Tab` / `Shift+Tab` - Switch between tabs
- `Ctrl+C` - Quit application

#### Chat Tab
- `Enter` - Send message
- `Ctrl+L` - Clear chat history
- `Ctrl+S` - Toggle system prompt display
- `Ctrl+Shift+C` - Copy conversation history to clipboard
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
| `chatModel` | Model to use for chat | `llama3.3:latest` |
| `embeddingModel` | Model to use for embeddings in RAG | `embeddinggemma:latest` |
| `ragEnabled` | Enable RAG (Retrieval Augmented Generation) | `true` |
| `ollamaURL` | URL of the Ollama server | `http://localhost:11434` |
| `chromaDBURL` | URL of the ChromaDB server | `http://localhost:8000` |
| `chromaDBDistance` | Distance threshold for similarity search | `1.0` |
| `maxDocuments` | Maximum documents to retrieve for RAG | `5` |
| `selectedCollections` | Selected collections for RAG queries | `{}` |
| `defaultSystemPrompt` | Default system prompt for conversations | (See configuration example) |

## Project Structure

```
gollama-chat/
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── configuration/          # Configuration management
│   │   ├── configuration.go
│   │   └── models/
│   │       └── models.go
│   ├── rag/                    # RAG (Retrieval Augmented Generation)
│   │   ├── service.go
│   │   └── service_test.go
│   └── tui/                    # Text User Interface
│       ├── ascii/
│       │   └── ascii.go
│       ├── core/
│       │   ├── tui.go         # Main TUI controller
│       │   ├── tui_test.go
│       │   └── height_31_test.go
│       ├── tabs/              # Tab implementations
│       │   ├── chat/
│       │   │   ├── chat.go    # Chat functionality
│       │   │   ├── chat_test.go
│       │   │   ├── messages.go
│       │   │   ├── message_cache.go
│       │   │   ├── model_context.go
│       │   │   ├── styles.go
│       │   │   ├── system_prompt.go
│       │   │   ├── token_counts.go
│       │   │   └── input/
│       │   │       ├── input.go
│       │   │       ├── input_test.go
│       │   │       └── PERFORMANCE_TESTING.md
│       │   ├── configuration/
│       │   │   ├── configuration.go # Settings tab
│       │   │   ├── models/
│       │   │   └── utils/
│       │   │       └── connection/
│       │   └── rag/
│       │       ├── rag.go
│       │       ├── collections_service.go
│       │       └── README.md
│       └── util/
│           ├── util.go
│           └── util_test.go
├── images/                    # Application images
│   ├── Icon.png
│   └── Screenshot.png
├── bin/                       # Built binaries
├── Makefile                   # Build automation
├── go.mod                     # Go module definition
├── TESTING.md                 # Testing documentation
├── TEST_COVERAGE_STRATEGY.md  # Coverage strategy
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
# Run all tests
make test

# Run tests with coverage report
make test-coverage

# Run performance regression tests
make test-performance

# Run input component benchmarks
make test-input-bench
```

#### Performance Testing

The project includes comprehensive performance testing to prevent input latency regressions:

- **Input Performance**: Benchmarks for typing responsiveness and character insertion
- **Regression Prevention**: Automated performance threshold monitoring
- **Memory Profiling**: Allocation pattern analysis and optimization
- **Real-world Scenarios**: Complex editing and Unicode handling tests

See [`internal/tui/tabs/chat/input/PERFORMANCE_TESTING.md`](internal/tui/tabs/chat/input/PERFORMANCE_TESTING.md) for detailed performance testing documentation.

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
- [ ] Streaming responses
- [ ] MCP Integration
