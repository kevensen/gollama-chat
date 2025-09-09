# RAG Collections Tab Documentation

## Overview
The RAG Collections tab provides an interface for managing ChromaDB collections used in Retrieval-Augmented Generation (RAG) functionality. This tab allows users to view, select, and manage collections stored in a ChromaDB instance.

## Features

### ✅ Implemented Features
- **Collection Listing**: Displays all collections from ChromaDB with scrollable interface
- **Connection Status**: Real-time connection status indicator with error messages
- **Collection Selection**: Select/deselect individual collections with immediate feedback
- **Bulk Operations**: Select all or deselect all collections with keyboard shortcuts
- **Collection Metadata**: Shows collection IDs and metadata count (expandable for future features)
- **Debug Tools**: Built-in debugging commands for troubleshooting
- **Configuration Management**: Live configuration refresh capabilities

### 🚀 Navigation & Controls

#### Basic Navigation
- **Tab/Shift+Tab**: Switch between application tabs
- **↑/↓ or k/j**: Navigate up/down through collections
- **Space/Enter**: Toggle selection of current collection
- **Ctrl+A**: Select all collections
- **Ctrl+D**: Deselect all collections
- **R**: Refresh collections list (when connected)

#### Debug & Management Commands
- **T**: Test ChromaDB connection
- **C**: Show current configuration (debug info)
- **U**: Refresh configuration from disk
- **X**: Cancel loading operations (emergency stop)

### 🔧 Connection Management

The RAG Collections tab uses the same ChromaDB URL configured in the Settings tab. The connection status is displayed at the top of the tab:

- **✓ Connected to ChromaDB** (Green): Successfully connected, collections can be loaded
- **✗ Not connected to ChromaDB** (Red): Connection failed or ChromaDB URL not configured

### 📋 Usage Instructions

1. **Configure ChromaDB URL**: Go to Settings tab and set the ChromaDB URL
2. **Navigate to RAG Collections**: Use Tab to switch to the RAG Collections tab
3. **Test Connection**: Press 'T' to test the ChromaDB connection
4. **View Collections**: Once connected, collections will load automatically
5. **Select Collections**: Use Space/Enter to toggle individual collections or Ctrl+A/Ctrl+D for bulk operations

### 🐛 Troubleshooting

If the RAG Collections tab shows "Loading..." or connection issues:

1. **Check Configuration**: Press 'C' to see the current ChromaDB URL
2. **Refresh Config**: Press 'U' to reload configuration from the Settings tab
3. **Test Connection**: Press 'T' to manually test the connection
4. **Emergency Stop**: Press 'X' if the tab gets stuck in loading state
5. **Verify Settings**: Switch to Settings tab and confirm ChromaDB connection is working

The RAG Collections tab provides a user interface for managing ChromaDB collections within the gollama-chat application. Users can view, select, and manage collections from their ChromaDB instance.

## Features

### ✅ Implemented Features

1. **Collections Listing**: Displays all collections available in the configured ChromaDB instance
2. **Connection Status**: Shows real-time connection status to ChromaDB
3. **Collection Selection**: Individual collection selection/deselection with space/enter
4. **Bulk Operations**: Select all (Ctrl+A) and deselect all (Ctrl+D) functionality
5. **Scrollable Interface**: Handles large numbers of collections with keyboard navigation
6. **Collection Metadata**: Displays collection IDs with room for future metadata expansion
7. **Default Selection**: All collections are selected by default when loaded
8. **Immediate Updates**: Changes are reflected immediately without disk persistence
9. **Error Handling**: Graceful handling of connection failures and errors

### Navigation Controls

- **↑/↓ or k/j**: Navigate up/down through collections
- **Space/Enter**: Toggle selection of current collection
- **Ctrl+A**: Select all collections
- **Ctrl+D**: Deselect all collections
- **R**: Refresh collections from ChromaDB
- **T**: Test ChromaDB connection
- **Tab/Shift+Tab**: Switch between application tabs

### UI Elements

- **Connection Status**: Green checkmark for connected, red X for disconnected
- **Collection List**: Scrollable list with selection indicators (● selected, ○ unselected)
- **Collection Info**: Name and ID displayed, with metadata count for future expansion
- **Selection Count**: Shows current selection status (e.g., "Selected: 3/5 collections")
- **Instructions**: Context-sensitive help text based on connection status

## File Structure

```
internal/tui/tabs/rag/
├── rag.go                    # Main RAG tab UI component
└── collections_service.go    # ChromaDB collections service
```

## Dependencies

- `github.com/amikos-tech/chroma-go` - ChromaDB client library
- `github.com/charmbracelet/bubbles/viewport` - Scrollable viewport component
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling library

## Configuration

The RAG Collections tab uses the ChromaDB URL configured in the Settings tab. Ensure:

1. ChromaDB server is running and accessible
2. ChromaDB URL is properly configured in Settings
3. Network connectivity to the ChromaDB instance

## Future Enhancements

The implementation includes infrastructure for future features:

- **Extended Metadata Display**: Collection metadata can be shown in detail
- **Collection Statistics**: Document counts, embedding models, etc.
- **Collection Management**: Create, delete, modify collections
- **Advanced Filtering**: Search and filter collections by criteria
- **Persistence**: Save selection state to configuration if needed

## Error Handling

- **Connection Failures**: UI disables and shows clear error messages
- **Timeout Handling**: 5-second connection timeout, 10-second operation timeout
- **Empty States**: Appropriate messages for no collections or disconnected state
- **Loading States**: Visual feedback during async operations

## Performance Considerations

- **Async Operations**: All ChromaDB operations run asynchronously
- **Efficient Rendering**: Only renders visible collections in viewport
- **Connection Pooling**: Reuses ChromaDB client connections
- **Memory Management**: Proper cleanup of resources

## Testing

To test the RAG Collections functionality:

1. Start a ChromaDB server
2. Configure the ChromaDB URL in Settings
3. Navigate to the RAG Collections tab
4. Verify connection status and collection listing
5. Test navigation and selection features
