# Table-Driven Unit Tests for gollama-chat

This document provides guidance and examples for implementing table-driven unit tests in the gollama-chat project, following Go best practices and patterns used in similar projects like fs-vectorize.

## Overview

Table-driven tests are a Go testing pattern that allows you to define multiple test cases in a structured way using a slice of test structs. This pattern is particularly useful for testing functions with multiple input/output scenarios.

## Benefits

- **Comprehensive Coverage**: Easy to add many test cases covering edge cases, boundary conditions, and normal scenarios
- **Maintainable**: Clear separation between test data and test logic
- **Readable**: Test cases are self-documenting with descriptive names and expected results
- **Efficient**: Shared test logic reduces code duplication

## Basic Pattern

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        expected OutputType
        expectError bool
        errorMsg string
    }{
        {
            name:     "descriptive test case name",
            input:    InputType{...},
            expected: OutputType{...},
            expectError: false,
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := FunctionName(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error but got none")
                }
                if tt.errorMsg != "" && err.Error() != tt.errorMsg {
                    t.Errorf("Expected error %q, got %q", tt.errorMsg, err.Error())
                }
                return
            }
            
            if err != nil {
                t.Errorf("Unexpected error: %v", err)
            }
            
            if result != tt.expected {
                t.Errorf("Expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

## Examples from gollama-chat

### 1. Configuration Validation Tests

**Location**: `internal/configuration/configuration_test.go`

```go
func TestConfig_Validate(t *testing.T) {
    tests := []struct {
        name        string
        config      *Config
        expectError bool
        errorMsg    string
    }{
        {
            name:        "valid default config",
            config:      DefaultConfig(),
            expectError: false,
        },
        {
            name: "empty ollama URL",
            config: &Config{
                ChatModel:        "llama3.3:latest",
                EmbeddingModel:   "embeddinggemma:latest",
                RAGEnabled:       true,
                OllamaURL:        "", // Empty - should fail
                ChromaDBURL:      "http://localhost:8000",
                ChromaDBDistance: 1.0,
                MaxDocuments:     5,
            },
            expectError: true,
            errorMsg:    "ollamaURL cannot be empty",
        },
        // More test cases for different validation scenarios...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()

            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error but got none")
                    return
                }
                if tt.errorMsg != "" && err.Error() != tt.errorMsg {
                    t.Errorf("Expected error message %q, got %q", tt.errorMsg, err.Error())
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error but got: %v", err)
                }
            }
        })
    }
}
```

### 2. RAG Service Tests

**Location**: `internal/rag/service_test.go`

```go
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
        // More test cases...
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
```

### 3. Token Estimation Tests

**Location**: `internal/tui/tabs/chat/token_counts_test.go`

```go
func TestEstimateTokens(t *testing.T) {
    tests := []struct {
        name         string
        text         string
        expectedMin  int
        expectedMax  int
        description  string
    }{
        {
            name:        "empty string",
            text:        "",
            expectedMin: 0,
            expectedMax: 0,
            description: "Empty string should return 0 tokens",
        },
        {
            name:        "simple sentence",
            text:        "Hello world",
            expectedMin: 2,
            expectedMax: 4,
            description: "Simple sentence should return 2-4 tokens",
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := estimateTokens(tt.text)
            
            if result < tt.expectedMin || result > tt.expectedMax {
                t.Errorf("estimateTokens(%q) = %d, expected between %d and %d (description: %s)", 
                    tt.text, result, tt.expectedMin, tt.expectedMax, tt.description)
            }
        })
    }
}
```

## Best Practices

### 1. Test Case Naming

- Use descriptive names that explain the scenario being tested
- Include the expected outcome in the name when helpful
- Examples:
  - `"valid default config"`
  - `"empty ollama URL"`
  - `"boundary ChromaDB distance values - minimum"`

### 2. Test Case Organization

```go
tests := []struct {
    name        string        // Required: describes the test case
    input       InputType     // Test input data
    expected    OutputType    // Expected result
    expectError bool         // Whether an error is expected
    errorMsg    string       // Specific error message (optional)
    description string       // Additional context (optional)
}{
    // Test cases organized by category:
    // 1. Valid cases first
    // 2. Invalid cases grouped by error type
    // 3. Boundary conditions
    // 4. Edge cases
}
```

### 3. Error Testing

Always test both success and failure scenarios:

```go
if tt.expectError {
    if err == nil {
        t.Errorf("Expected error but got none")
        return
    }
    // Optionally check specific error message
    if tt.errorMsg != "" && err.Error() != tt.errorMsg {
        t.Errorf("Expected error message %q, got %q", tt.errorMsg, err.Error())
    }
} else {
    if err != nil {
        t.Errorf("Expected no error but got: %v", err)
    }
}
```

### 4. Boundary Value Testing

Include tests for boundary conditions:

```go
{
    name: "boundary ChromaDB distance values - minimum",
    config: &Config{
        ChromaDBDistance: 0.0, // Minimum valid value
        // ... other fields
    },
    expectError: false,
},
{
    name: "boundary ChromaDB distance values - maximum",
    config: &Config{
        ChromaDBDistance: 2.0, // Maximum valid value
        // ... other fields
    },
    expectError: false,
},
```

### 5. Helper Functions

For complex test setup, use helper functions:

```go
// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func createTestConfig(overrides map[string]interface{}) *Config {
    config := DefaultConfig()
    // Apply overrides...
    return config
}
```

## Running Tests

The Makefile provides several test targets:

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with coverage
make test-coverage

# Run specific package tests
make test-config    # Configuration tests
make test-rag       # RAG service tests
make test-chat      # Chat functionality tests

# Run benchmarks
make test-bench

# Quick test run (skip long-running tests)
make test-short
```

## Coverage Reporting

Generate test coverage reports:

```bash
# HTML coverage report
make test-coverage

# Terminal coverage summary
make coverage-terminal
```

## Benchmarking

Include benchmarks for performance-critical functions:

```go
func BenchmarkEstimateTokens(b *testing.B) {
    testCases := []struct {
        name string
        text string
    }{
        {"short", "Hello world"},
        {"medium", "This is a medium length sentence..."},
        {"long", "This is a very long piece of text..."},
    }

    for _, tc := range testCases {
        b.Run(tc.name, func(b *testing.B) {
            for i := 0; i < b.N; i++ {
                estimateTokens(tc.text)
            }
        })
    }
}
```

## Testing Patterns for Different Components

### Configuration Functions
- Test default values
- Test validation rules
- Test edge cases and boundary conditions
- Test OS-specific behavior (file paths, environment variables)

### Service Functions
- Test initialization states
- Test state transitions
- Test error conditions
- Test with different configurations

### Utility Functions
- Test with empty/nil inputs
- Test with typical inputs
- Test with edge cases
- Test performance characteristics

### TUI Components
- Test message handling
- Test state updates
- Test view rendering (where testable)
- Test user input validation

## Integration with Existing Patterns

This project follows patterns established in the fs-vectorize project:

1. **Comprehensive test coverage** with table-driven tests
2. **Clear test organization** by package and functionality
3. **Consistent error testing** patterns
4. **Performance benchmarking** for critical paths
5. **Documentation** of test patterns and expectations

## Adding New Tests

When adding new functions to gollama-chat:

1. **Create corresponding test file**: `filename_test.go` in the same package
2. **Use table-driven pattern** for functions with multiple scenarios
3. **Include error cases** and boundary conditions
4. **Add benchmarks** for performance-critical functions
5. **Update Makefile** if adding new test packages
6. **Document complex test scenarios** in code comments

## Conclusion

Table-driven tests provide a robust foundation for ensuring code quality in gollama-chat. By following these patterns and best practices, you can create comprehensive, maintainable test suites that catch bugs early and support confident refactoring.

For more examples, see the existing test files in the fs-vectorize project and the newly created test files in this gollama-chat implementation.