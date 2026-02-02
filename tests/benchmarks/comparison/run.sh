#!/bin/bash
# Benchmark runner for Risor vs TypeScript comparison
# Usage: ./run.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== Risor vs TypeScript Performance Comparison ==="
echo ""

# Check dependencies
if ! command -v risor &> /dev/null; then
    echo "Error: risor CLI not found. Install with: cd cmd/risor && go install ."
    exit 1
fi

BUN="${HOME}/.bun/bin/bun"
if ! command -v bun &> /dev/null && [[ ! -x "$BUN" ]]; then
    echo "Error: bun not found. Install from https://bun.sh"
    exit 1
fi
command -v bun &> /dev/null && BUN="bun"

run_benchmark() {
    local name=$1
    local risor_file="${name}.risor"
    local ts_file="${name}.ts"

    echo "--- ${name} ---"

    # Run Risor
    echo -n "Risor:      "
    TIMEFORMAT='%3R seconds'
    { time risor "$risor_file" > /dev/null; } 2>&1

    # Run TypeScript (using bun for fast startup)
    echo -n "TypeScript: "
    { time $BUN "$ts_file" > /dev/null; } 2>&1

    echo ""
}

# Run each benchmark
run_benchmark "01_fibonacci"
run_benchmark "02_array_ops"
run_benchmark "03_closures"
run_benchmark "04_recursion"
run_benchmark "05_higher_order"

echo "=== Done ==="
