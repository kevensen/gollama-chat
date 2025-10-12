# AGENTS.md
This is a project that is CLI/TUI chat bot for Ollama
# Code Guidelines
## Golang
When writing Golang, both best practices and style decisions should be followed wherever possible
### Best Practices
Golang Best Practices are defined at https://google.github.io/styleguide/go/best-practices
### Decisions
Style decisions are defined at https://google.github.io/styleguide/go/decisions
# Hints
1. Do not do multi-line concatenation in a file, either locally or remote.  It doesn't work well in the terminal
## Functionality
### Tool authorization
1. The invocation of tools shall require user consent.
2. The assistant shall tell the user it wants to use a specific tool by using the tools name and prompt the user y/n/t.
  - y indicates the tool use for that specific invocation
  - n indicates the tool should not be used
  - t indicates the tool should be used for that specific invocation and any additional invocations for remainder of the session
3. The input box shall indicate "Type your response..." instead of "Type your questions...".
4. The user will respond in the "input".
5. The use of RAG shall never require user permission.

#### Trust Levels
The system implements a 3-level trust system for tools:
- **0 (TrustNone)**: Blocks tool execution entirely with message directing user to Tools tab
- **1 (AskForTrust)**: Prompts for user permission (default for new tools)
- **2 (TrustSession)**: Allows execution without prompting for the session

#### Unified Tool System
- Both builtin and MCP tools use the same authorization framework
- MCP tools are namespaced as `server.toolname` to avoid conflicts
- Tool availability is checked (especially important for MCP tools when servers are down)
- Trust levels persist in configuration and can be managed via the Tools tab

#### Permission Prompt Format
```
❓ Tool 'toolname' wants to execute with arguments: {...}

Allow execution? (y)es / (n)o / (t)rust for session
```
### MCP
1. User request: The flow begins with a user issuing a natural-language request to an AI agent, often through a chat interface. For example, "Generate a report summarizing sales in Q3".
2. Tool matching: An MCP client, which is embedded within the user's application (e.g., a code editor like Cursor or a platform like Langflow), matches the user's intent to the capabilities of available MCP servers.
3. Tool permission: gollama-chat should prompt the user for permission to execute the tool
4. Tool invocation: The MCP client orchestrates the call to the relevant MCP server. It packages the request and any necessary parameters, such as a date range for the sales report, into a standardized JSON-RPC message.
5. Server execution: The MCP server receives the request and executes the corresponding tool or function. For a sales report, this might involve querying an internal database and running a data pipeline.
6. Response handling: The MCP server formats the result of the tool's execution into a standardized schema and sends it back to the client.
7. AI analysis: The AI agent processes the results from the MCP server, potentially performing further analysis or summarizing the data into an easy-to-understand response.
8. Final response: The AI agent provides the final, natural-language response to the user.

#### MCP Implementation Details
- MCP tools are managed through a unified tool registry alongside builtin tools
- MCP tools use the same 3-level trust system as builtin tools
- MCP tools are automatically namespaced as `server.toolname` format
- Server availability is monitored - unavailable tools show appropriate error messages
- Tool discovery happens during MCP server initialization via JSON-RPC `tools/list` calls
- Tool execution uses JSON-RPC `tools/call` with proper error handling
- MCP servers can be started/stopped independently and tools become available/unavailable accordingly 
## Logs
Logs should be accessed for debugging.
### Logging Chat
When a user enters a prompt/question, a ULID is created.  This same ULID should be used when logging anything related to the flow of the prompt (e.g. tool permission requests, a tool permission response from the user, or final response from the LLM).  A new ULID should not be created for each new log entry.  Only when the user enters a new question.

## Tests
### Verification
DO build table-driven unit tests for new functionality.
DO keep unit tests up to date with changed functionality.
### Validation
* DO NOT rely on simply building the binary for validation.
* DO execute unit-tests (`make test`)
