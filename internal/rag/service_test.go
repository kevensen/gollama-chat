package rag

import (
"testing"

"github.com/kevensen/gollama-chat/internal/configuration"
)

func TestService_IsReady(t *testing.T) {
tests := []struct {
name                string
ragEnabled          bool
connected           bool
selectedCollections []string
expected            bool
}{
{
name:                "ready - all conditions met",
ragEnabled:          true,
connected:           true,
selectedCollections: []string{"collection1", "collection2"},
expected:            true,
},
{
name:                "not ready - RAG disabled",
ragEnabled:          false,
connected:           true,
selectedCollections: []string{"collection1"},
expected:            false,
},
{
name:                "not ready - not connected",
ragEnabled:          true,
connected:           false,
selectedCollections: []string{"collection1"},
expected:            false,
},
{
name:                "not ready - no collections selected",
ragEnabled:          true,
connected:           true,
selectedCollections: []string{},
expected:            false,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
config := &configuration.Config{
RAGEnabled: tt.ragEnabled,
}

service := NewService(config)
service.connected = tt.connected
service.selectedCollections = tt.selectedCollections

result := service.IsReady()
if result != tt.expected {
t.Errorf("IsReady() = %v, expected %v", result, tt.expected)
}
})
}
}

func TestNewService(t *testing.T) {
config := configuration.DefaultConfig()
service := NewService(config)

if service == nil {
t.Fatal("NewService returned nil")
}

if service.config != config {
t.Error("Service config should reference the provided config")
}

if service.connected {
t.Error("New service should not be connected initially")
}

if service.selectedCollections == nil {
t.Error("selectedCollections should be initialized")
}

if len(service.selectedCollections) != 0 {
t.Error("selectedCollections should be empty initially")
}
}
