# Goal
Create a text user interface (TUI) for chatting with an LLM.

# Guidance
1. Use the https://github.com/charmbracelet/bubbletea for the text user interface
2. Obey the principles of high cohesion and loose coupling
3. Views should be compostable
4. The UI will have multiple tabs.
5. Eventually use https://github.com/amikos-tech/chroma-go for interaction with Chromadb servers.  This will come later.
6. Favor idiomatic Go where possible.
7. Create additional directories and *.go files as deemed necessary.

# Configuration

The application stores its configuration in a JSON file located at:
- **Linux/macOS**: `~/.config/gollama/settings.json`
- **Windows**: `%APPDATA%\gollama\settings.json`

# Direction
Complete work items.  When work items are complete, move them from "Todo" to "Done"

# Work Items
## Todo
15. The indicator of connectivity to Ollama should be in line with the "Ollama URL".
16. The indicator of connectivity to the ChromaDB server should be in line with the "ChromaDB URL".

## Done
1. ✅ Define the main TUI controller in /home/kdevensen/workspace/gollama-chat/internal/tui/tui/tui.go.
2. ✅ Configuration should be read from/written to the aforementioned configuration files.  The model of this configuration should be in the same file.  The mechanics of this should be done in /home/kdevensen/workspace/gollama-chat/internal/configuration/configuration.go.
3. ✅ The modification of the configuration should be done in a "settings" TUI tab.  The view should be defined in /home/kdevensen/workspace/gollama-chat/internal/tui/tabs/configuration/configuration.go
4. ✅ The chat functionality should be in a "chat" TUI tab.  The view should be defined /home/kdevensen/workspace/gollama-chat/internal/tui/tabs/chat/chat.go.
5. ✅ On the "chat" tab, there should be a text area at the bottom of the view.
6. ✅ On the "chat" tab, at the top of the view there should be an area that displays the alternating messages between the user and the "assistant".
7. ✅ Ensure the size of the viewable area is less than size of the terminal window.  The width is 95% the width of the terminal.  The height is 95% of the height.
8. ✅ The sizing of the application window still seems off.  I am unable to view the tabs at the top. The border of the text entry area at the bottom seems to wrap around.  Please reduce the area of the application.
9. ✅ Ensure that the "settings" tab obeys the same sizing calculations/rules as the "chat" tab.  When I hit "tab" to select the "settings" tab, I am unable to see the tabs at the top.  I am unable to see the configuration items.
10. ✅ The configuration items should be based off the following JSON:
{
  "lastModel": "llama3.3:latest",
  "embeddingModel": "embeddinggemma:latest", 
11. ✅ In the settings, I want to change the "Last Model/lastModel" to "Chat Model/chatModel". 
  "ragEnabled": true,
  "ollamaURL": "http://192.168.86.232:11434",
  "chromaDBURL": "http://172.16.194.130:8000",
  "chromaDBDistance": 0.95,
  "maxDocuments": 5,
  "darkMode": false,
  "selectedCollections": {
    "20250908_174013": true
  }
}
12. ✅ On the "settings" tab, provide an indication that the connection to the Ollama server is successful or unsuccessful.
13. ✅ On the "settings" tab, provide an indication that a connection to the Chromadb server is successful or unsuccessful.  Use the `/api/v2` endpoint to make this determination.
14. ✅ On the settings tab, when either the "Chat Model" or "Embedding Model" are selected, a bordered panel to the right lists available models as provided by the Ollama server. The user can navigate and scroll to select a desired model. If a connection to the Ollama server cannot be established, a message is displayed indicating as much.
