#!/bin/bash
set -e

# TODO: need to make this work with a release URL instead of local paths

# Generate Go bindings for EVM contract errors with selector-based decoding
# This script:
# 1. Extracts error ABIs from compiled Solidity contracts
# 2. Runs a Go code generator to create error types, decoders, and helpers
# 3. Generates a selector-to-decoder map for runtime error parsing

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CONTRACTS_DIR="$ROOT_DIR/service_contracts"
OUT_DIR="$CONTRACTS_DIR/out"
PKG_DIR="$ROOT_DIR/pkg/evmerrors"
GENERATOR_DIR="$SCRIPT_DIR/error-binding-generator"

echo "==> Generating EVM error bindings..."
echo ""

# Check dependencies
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "Error: go is required but not installed"
    exit 1
fi

# Ensure contracts are built
if [ ! -d "$OUT_DIR" ]; then
    echo "Error: Contracts not built. Running 'make build' in service_contracts/"
    cd "$CONTRACTS_DIR"
    make build
    cd "$ROOT_DIR"
fi

# Define error ABI sources
ERRORS_ABI_1="$OUT_DIR/Errors.sol/Errors.json"
ERRORS_ABI_2="$CONTRACTS_DIR/lib/fws-payments/out/Errors.sol/Errors.json"

# Check if main Errors.sol is compiled
if [ ! -f "$ERRORS_ABI_1" ]; then
    echo "Error: Errors.sol not compiled. Run 'make build' in service_contracts/ first"
    exit 1
fi

# Check if payments Errors.sol is compiled
PAYMENTS_BUILT=false
if [ -f "$ERRORS_ABI_2" ]; then
    PAYMENTS_BUILT=true
    echo "Found Payments errors at: $ERRORS_ABI_2"
else
    echo "Warning: Payments Errors.sol not found at: $ERRORS_ABI_2"
    echo "Only generating bindings for service_contracts/src/Errors.sol"
fi

# Create temp directory for merging
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Extract errors from main Errors.sol
echo "==> Extracting errors from service_contracts/src/Errors.sol..."
jq '[.abi[] | select(.type == "error")]' "$ERRORS_ABI_1" > "$TEMP_DIR/errors1.json"
ERROR_COUNT_1=$(jq 'length' "$TEMP_DIR/errors1.json")
echo "Found $ERROR_COUNT_1 errors in Errors.sol"

# Extract errors from payments if available
if [ "$PAYMENTS_BUILT" = true ]; then
    echo "==> Extracting errors from fws-payments/src/Errors.sol..."
    jq '[.abi[] | select(.type == "error")]' "$ERRORS_ABI_2" > "$TEMP_DIR/errors2.json"
    ERROR_COUNT_2=$(jq 'length' "$TEMP_DIR/errors2.json")
    echo "Found $ERROR_COUNT_2 errors in Payments Errors.sol"

    # Merge and deduplicate by signature (name + param types)
    echo "==> Merging and deduplicating errors..."
    jq -s 'add | unique_by(.name + "(" + ([.inputs[].type] | join(",")) + ")")' \
        "$TEMP_DIR/errors1.json" "$TEMP_DIR/errors2.json" > "$TEMP_DIR/merged_errors.json"
else
    # Just use the first file
    cp "$TEMP_DIR/errors1.json" "$TEMP_DIR/merged_errors.json"
fi

# Count final merged errors
TOTAL_ERRORS=$(jq 'length' "$TEMP_DIR/merged_errors.json")
echo "Total unique errors after merge: $TOTAL_ERRORS"

# Wrap in ABI structure for generator
jq '{abi: .}' "$TEMP_DIR/merged_errors.json" > "$TEMP_DIR/merged_abi.json"
echo ""

# Build the code generator
echo "==> Building error binding generator..."
cd "$GENERATOR_DIR"
if [ ! -f "go.mod" ]; then
    echo "Error: go.mod not found in $GENERATOR_DIR"
    exit 1
fi

# Download dependencies
go mod download
go mod tidy

# Build the generator
go build -o generator main.go
echo "Generator built successfully"
echo ""

# Create output directory
mkdir -p "$PKG_DIR"

# Run the generator on merged ABIs
echo "==> Running error binding generator on merged errors..."
./generator -abi "$TEMP_DIR/merged_abi.json" -out "$PKG_DIR"
echo ""

# Clean up
rm -f generator

echo "==> Generated files in $PKG_DIR:"
ls -lh "$PKG_DIR"/*.go
echo ""

# Create go.mod for the package if it doesn't exist
if [ ! -f "$PKG_DIR/go.mod" ]; then
    echo "==> Creating go.mod for evmerrors package..."
    cd "$PKG_DIR"
    go mod init github.com/storacha/filecoin-services/pkg/evmerrors
    go mod tidy
    cd "$ROOT_DIR"
fi

echo "✓ EVM error bindings generated successfully!"
echo ""
echo "Usage in Go code:"
echo "  import \"github.com/storacha/filecoin-services/pkg/evmerrors\""
echo ""
echo "  revertData := \"0x6c577bf9000...\""
echo "  err, parseErr := evmerrors.ParseRevert(revertData)"
echo "  if evmerrors.IsInvalidEpochRange(err) {"
echo "      // Handle InvalidEpochRange error"
echo "  }"
echo ""
echo "Regenerate after contract changes with:"
echo "  ./tools/generate-evm-error-bindings.sh"
echo ""
