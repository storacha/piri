#!/bin/bash

# Script to parse Go test output and highlight failures
# Usage: ./scripts/parse-test-failures.sh [test-output-file]

set -euo pipefail

TEST_OUTPUT="${1:-test-output.log}"

if [[ ! -f "$TEST_OUTPUT" ]]; then
    echo "❌ Test output file '$TEST_OUTPUT' not found!"
    echo "Usage: $0 [test-output-file]"
    exit 1
fi

echo "🔍 Parsing test failures from: $TEST_OUTPUT"
echo "================================================="

# Check if there are any failures
if ! grep -q "FAIL" "$TEST_OUTPUT"; then
    echo "✅ No test failures found!"
    exit 0
fi

echo ""
echo "🚨 FAILED TESTS SUMMARY:"
echo "------------------------"

# Extract failed test names
grep -E "--- FAIL:" "$TEST_OUTPUT" | while read -r line; do
    test_name=$(echo "$line" | awk '{print $3}' | sed 's/(.*//')
    package=$(echo "$line" | grep -o '([^)]*)')
    echo "  ❌ $test_name $package"
done

echo ""
echo "🔥 ERROR MESSAGES:"
echo "------------------"

# Extract error messages with context
grep -A 3 -B 1 -E "(panic:|Error:|error:|FAIL.*Test|assertion failed)" "$TEST_OUTPUT" | \
    grep -v "^--$" | \
    sed 's/^/  /' | \
    head -30

echo ""
echo "📊 FAILURE STATISTICS:"
echo "----------------------"

total_tests=$(grep -c "=== RUN" "$TEST_OUTPUT" || echo "0")
failed_tests=$(grep -c "--- FAIL:" "$TEST_OUTPUT" || echo "0")
passed_tests=$((total_tests - failed_tests))

echo "  Total tests: $total_tests"
echo "  Passed: $passed_tests"
echo "  Failed: $failed_tests"

if [[ $failed_tests -gt 0 ]]; then
    echo ""
    echo "💡 TIPS:"
    echo "--------"
    echo "  • Run specific failed test: go test -v ./path/to/package -run TestName"
    echo "  • Enable race detection: go test -race ./..."
    echo "  • Increase verbosity: go test -v ./..."
    echo "  • Full output available in: $TEST_OUTPUT"
fi

exit 1
