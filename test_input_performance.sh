#!/bin/bash

# Performance Testing Script for Input Component
# This script runs performance benchmarks and checks for regressions

set -e

echo "🚀 Running Input Performance Benchmarks..."
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Define performance thresholds (nanoseconds per operation)
# These are based on current baseline performance
CRITICAL_THRESHOLDS=(
    "BenchmarkInsertCharacterDirect:1000"              # Should be under 1000ns
    "BenchmarkUpdate/key_rune_ascii:200"               # Should be under 200ns  
    "BenchmarkTypingSequence/short_question:2000"      # Should be under 2000ns
    "BenchmarkRealWorldTyping/type_and_correct:3000"   # Should be under 3000ns
    "BenchmarkFullInputPipeline/empty_input_ascii:40000" # Full pipeline should be under 40000ns
    "BenchmarkFullInputPipeline/short_text_append:35000" # Full pipeline append should be under 35000ns
    "BenchmarkFastPathEfficiency/fast_path_ascii:35000"  # Fast path should be under 35000ns
)

# Run benchmarks and capture output
echo "Running input component benchmarks..."
INPUT_BENCH_OUTPUT=$(cd /home/kdevensen/workspace/gollama-chat && go test -bench=. -benchmem ./internal/tui/tabs/chat/input/ 2>&1)

echo "Running full pipeline benchmarks..."
PIPELINE_BENCH_OUTPUT=$(cd /home/kdevensen/workspace/gollama-chat && go test -bench=BenchmarkFullInputPipeline -benchmem ./internal/tui/tui/ 2>&1)

echo "Running fast path efficiency benchmarks..."
EFFICIENCY_BENCH_OUTPUT=$(cd /home/kdevensen/workspace/gollama-chat && go test -bench=BenchmarkFastPathEfficiency -benchmem ./internal/tui/tui/ 2>&1)

# Combine all outputs
BENCH_OUTPUT="$INPUT_BENCH_OUTPUT
$PIPELINE_BENCH_OUTPUT
$EFFICIENCY_BENCH_OUTPUT"

echo "$BENCH_OUTPUT"

# Check if benchmarks passed
if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Benchmarks failed to run${NC}"
    exit 1
fi

echo ""
echo "🔍 Performance Analysis:"
echo "========================"

# Parse benchmark results and check thresholds
WARNINGS=0
FAILURES=0

for threshold in "${CRITICAL_THRESHOLDS[@]}"; do
    IFS=':' read -r benchmark_name max_ns <<< "$threshold"
    
    # Extract the performance result for this benchmark
    result_line=$(echo "$BENCH_OUTPUT" | grep "^$benchmark_name" | head -1)
    
    if [ -n "$result_line" ]; then
        # Extract ns/op value (format: BenchmarkName-16    iterations    ns/op    B/op    allocs/op)
        ns_per_op=$(echo "$result_line" | awk '{print $3}' | sed 's/ns\/op//')
        
        # Convert to integer for comparison (remove decimal)
        ns_per_op_int=$(echo "$ns_per_op" | cut -d'.' -f1)
        
        if [ "$ns_per_op_int" -gt "$max_ns" ]; then
            echo -e "${RED}❌ PERFORMANCE REGRESSION: $benchmark_name${NC}"
            echo -e "   Current: ${ns_per_op}ns/op | Threshold: ${max_ns}ns/op"
            FAILURES=$((FAILURES + 1))
        elif [ "$ns_per_op_int" -gt $((max_ns * 80 / 100)) ]; then
            echo -e "${YELLOW}⚠️  WARNING: $benchmark_name approaching threshold${NC}"
            echo -e "   Current: ${ns_per_op}ns/op | Threshold: ${max_ns}ns/op"
            WARNINGS=$((WARNINGS + 1))
        else
            echo -e "${GREEN}✅ $benchmark_name: ${ns_per_op}ns/op (threshold: ${max_ns}ns/op)${NC}"
        fi
    else
        echo -e "${RED}❌ Could not find benchmark result for: $benchmark_name${NC}"
        FAILURES=$((FAILURES + 1))
    fi
done

echo ""
echo "📊 Key Performance Metrics:"
echo "============================"

# Extract and display key metrics
echo "Critical Path Performance:"
echo -n "  • Character insertion (component): "
echo "$BENCH_OUTPUT" | grep "BenchmarkInsertCharacterDirect/append_to_short" | awk '{print $3}'

echo -n "  • ASCII key handling (component): "
echo "$BENCH_OUTPUT" | grep "BenchmarkUpdate/key_rune_ascii" | awk '{print $3}'

echo -n "  • Short message typing (component): "
echo "$BENCH_OUTPUT" | grep "BenchmarkTypingSequence/short_question" | awk '{print $3}'

echo -n "  • Full pipeline - empty input: "
echo "$BENCH_OUTPUT" | grep "BenchmarkFullInputPipeline/empty_input_ascii" | awk '{print $3}'

echo -n "  • Full pipeline - text append: "
echo "$BENCH_OUTPUT" | grep "BenchmarkFullInputPipeline/short_text_append" | awk '{print $3}'

echo -n "  • Fast path efficiency: "
echo "$BENCH_OUTPUT" | grep "BenchmarkFastPathEfficiency/fast_path_ascii" | awk '{print $3}'

echo ""
echo "Memory Usage:"
echo -n "  • Character insertion memory (component): "
echo "$BENCH_OUTPUT" | grep "BenchmarkInsertCharacterDirect/append_to_short" | awk '{print $4, $5}'

echo -n "  • ASCII key memory (component): "
echo "$BENCH_OUTPUT" | grep "BenchmarkUpdate/key_rune_ascii" | awk '{print $4, $5}'

echo -n "  • Full pipeline memory: "
echo "$BENCH_OUTPUT" | grep "BenchmarkFullInputPipeline/empty_input_ascii" | awk '{print $4, $5}'

echo ""
echo "📈 Summary:"
echo "==========="

if [ "$FAILURES" -gt 0 ]; then
    echo -e "${RED}❌ PERFORMANCE TEST FAILED${NC}"
    echo "   Failures: $FAILURES"
    echo "   Warnings: $WARNINGS"
    echo ""
    echo "Action required: Input performance has regressed beyond acceptable thresholds."
    echo "Please investigate and optimize the failing components before merging."
    exit 1
elif [ "$WARNINGS" -gt 0 ]; then
    echo -e "${YELLOW}⚠️  PERFORMANCE WARNINGS DETECTED${NC}"
    echo "   Failures: $FAILURES"
    echo "   Warnings: $WARNINGS"
    echo ""
    echo "Consider reviewing performance for components approaching thresholds."
    exit 0
else
    echo -e "${GREEN}✅ ALL PERFORMANCE TESTS PASSED${NC}"
    echo "   Failures: $FAILURES"
    echo "   Warnings: $WARNINGS"
    echo ""
    echo "Input performance is within acceptable bounds."
    exit 0
fi