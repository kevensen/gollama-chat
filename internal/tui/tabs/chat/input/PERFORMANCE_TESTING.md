# Input Performance Testing

This directory contains comprehensive performance testing for the input component and full input pipeline to prevent typing latency regressions.

## Background

Before implementing unit tests, there was considerable latency when typing in the input pane, causing keystrokes to be missed if users typed too fast. After debugging, this was identified as a rendering performance issue. The issue has been resolved, and this benchmark suite ensures it doesn't regress.

## Testing Scope

### Component-Level Testing
- `input_bench_test.go` - Input component benchmarks in isolation

### Full Pipeline Testing  
- `../../tui/tui_input_bench_test.go` - Complete input path benchmarks
- Tests the entire flow: `TUI.Update` → `Chat.Update` → `Input.InsertCharacterDirect`

## Files

- `input_bench_test.go` - Input component benchmarks
- `../../tui/tui_input_bench_test.go` - Full pipeline benchmarks
- `../../test_input_performance.sh` - Performance regression testing script

## Running Performance Tests

### Quick Performance Check
```bash
make test-performance
```

### Component-Only Benchmarks
```bash
make test-input-bench
```

### Full Pipeline Benchmarks
```bash
# All TUI input pipeline benchmarks
go test -bench=. -benchmem ./internal/tui/tui/

# Specific pipeline benchmark
go test -bench=BenchmarkFullInputPipeline -benchmem ./internal/tui/tui/

# Fast path efficiency comparison
go test -bench=BenchmarkFastPathEfficiency -benchmem ./internal/tui/tui/
```

### Manual Benchmark Commands
```bash
# All benchmarks with memory profiling
go test -bench=. -benchmem ./internal/tui/tabs/chat/input/

# Specific benchmark
go test -bench=BenchmarkInsertCharacterDirect ./internal/tui/tabs/chat/input/

# CPU profiling for analysis
go test -bench=BenchmarkTypingSequence -cpuprofile=cpu.prof ./internal/tui/tabs/chat/input/
```

## Performance Thresholds

The performance script monitors these critical thresholds:

| Operation | Threshold | Description |
|-----------|-----------|-------------|
| **Component Level** | | |
| Character insertion | < 1000ns/op | Core typing responsiveness |
| ASCII key handling | < 200ns/op | Event processing speed |
| Short message typing | < 2000ns/op | Realistic typing scenarios |
| Real-world corrections | < 3000ns/op | Complex editing patterns |
| **Full Pipeline** | | |
| Empty input typing | < 40000ns/op | Complete TUI→Chat→Input path |
| Text append typing | < 35000ns/op | Typing with existing text |
| Fast path efficiency | < 35000ns/op | ASCII fast path performance |

## Benchmark Categories

### 1. Component-Level Operations
- **BenchmarkInsertCharacterDirect**: Direct character insertion (critical path)
- **BenchmarkBackspaceOperations**: Delete operations across text lengths
- **BenchmarkCursorMovement**: Navigation responsiveness
- **BenchmarkUpdate**: Event handling performance

### 2. Full Pipeline Operations
- **BenchmarkFullInputPipeline**: Complete TUI→Chat→Input flow
- **BenchmarkFastPathEfficiency**: ASCII fast path vs normal path comparison
- **BenchmarkTabSwitchingImpact**: Performance when different tabs are active
- **BenchmarkWindowSizeUpdates**: Impact of window resizing during typing

### 3. Realistic Scenarios
- **BenchmarkTypingSequence**: Complete typing workflows (component)
- **BenchmarkRealWorldTyping**: Complex editing with corrections (component)
- **BenchmarkRealisticTypingWorkload**: Real-world patterns through full pipeline
- **BenchmarkUnicodeHandling**: Multi-byte character support

### 4. System Performance
- **BenchmarkView**: Rendering optimization
- **BenchmarkMemoryAllocation**: Memory usage patterns (both levels)

## Integration with CI/CD

Add to your CI pipeline:
```yaml
- name: Performance Regression Test
  run: ./test_input_performance.sh
```

The script will fail if performance degrades beyond acceptable thresholds, preventing regressions from reaching production.

## Analyzing Results

### Understanding Output
```
BenchmarkInsertCharacterDirect/append_to_short-16    1879593    615.1 ns/op    224 B/op    3 allocs/op
```

- `1879593`: Number of iterations
- `615.1 ns/op`: Nanoseconds per operation
- `224 B/op`: Bytes allocated per operation
- `3 allocs/op`: Memory allocations per operation

### Performance Analysis
- **ns/op**: Lower is better for responsiveness
- **B/op**: Lower is better for memory efficiency
- **allocs/op**: Lower is better for GC pressure

### Profiling for Optimization
```bash
# CPU profile analysis
go test -bench=BenchmarkTypingSequence -cpuprofile=cpu.prof ./internal/tui/tabs/chat/input/
go tool pprof cpu.prof

# Memory profile analysis
go test -bench=BenchmarkMemoryAllocation -memprofile=mem.prof ./internal/tui/tabs/chat/input/
go tool pprof mem.prof
```

## Best Practices

1. **Run benchmarks consistently** - Same hardware, minimal background processes
2. **Monitor trends** - Track performance over time, not just single runs
3. **Profile regressions** - Use pprof to understand performance changes
4. **Test realistic scenarios** - Include real user typing patterns
5. **Consider memory pressure** - Monitor allocations, not just speed