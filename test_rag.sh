#!/bin/bash

echo "=== RAG Functionality Test Script ==="
echo
echo "This script helps test if RAG is working correctly in gollama-chat."
echo
echo "Prerequisites:"
echo "1. ChromaDB server should be running"
echo "2. ChromaDB should have collections with documents"
echo "3. The application should be configured with:"
echo "   - ChromaDB URL (e.g., http://localhost:8000)"
echo "   - RAG enabled"
echo "   - Ollama URL for embeddings"
echo
echo "Testing Steps:"
echo "1. Run the application: ./bin/gollama-chat"
echo "2. Go to Settings tab and ensure:"
echo "   - ChromaDB URL is set"
echo "   - RAG is enabled"
echo "   - Save the configuration"
echo "3. Go to RAG Collections tab and:"
echo "   - Verify connection status shows 'Connected'"
echo "   - Check that collections are listed"
echo "   - Ensure desired collections are selected (● means selected)"
echo "4. Go to Chat tab and send a message"
echo "5. Check the terminal output for RAG debug messages:"
echo "   - 'RAG: Retrieved X documents' = SUCCESS"
echo "   - 'RAG: No relevant documents found' = No matching docs"
echo "   - 'RAG query error: ...' = Error occurred"
echo "   - 'RAG: Service not ready' = Configuration issue"
echo
echo "Common Issues:"
echo "- If you see 'no collections selected for RAG':"
echo "  * Go to RAG Collections tab"
echo "  * Select collections using Space/Enter"
echo "  * Switch back to Chat tab"
echo "- If you see 'Connection failed':"
echo "  * Check ChromaDB server is running"
echo "  * Verify ChromaDB URL in Settings"
echo "- If you see 'RAG: Service not ready':"
echo "  * Check RAG is enabled in Settings"
echo "  * Verify collections are selected in RAG tab"
echo
echo "Starting the application..."
echo "Monitor the terminal output for RAG debug messages when you send chat messages."
echo

# Check if ChromaDB is accessible
if command -v curl &> /dev/null; then
    CHROMADB_URL="http://localhost:8000"
    echo "Testing ChromaDB connection at $CHROMADB_URL..."
    if curl -s "$CHROMADB_URL/api/v1/heartbeat" > /dev/null 2>&1; then
        echo "✓ ChromaDB appears to be running at $CHROMADB_URL"
    else
        echo "✗ ChromaDB does not appear to be running at $CHROMADB_URL"
        echo "  Please start ChromaDB before testing RAG functionality."
    fi
    echo
fi

exec ./bin/gollama-chat
