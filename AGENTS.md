# Agents
This is a project that is CLI/TUI chat bot for Ollama
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
## Tests
DO build table-driven unit tests for new functionality.
DO keep unit tests up to date with changed functionality.
## Validation
DO NOT rely on simply building the binary for validation.
DO execute unit-tests (`make tests`)