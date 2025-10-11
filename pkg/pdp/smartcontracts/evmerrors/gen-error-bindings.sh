#!/bin/bash
set -e

# TODO: need to make this work with a release URL instead of local paths

# Generate Go bindings for Solidity custom errors
# This script extracts error ABIs from compiled contracts and generates Go code using abigen

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CONTRACTS_DIR="$ROOT_DIR/service_contracts"
OUT_DIR="$CONTRACTS_DIR/out"
PKG_DIR="$ROOT_DIR/pkg/fvmerrors"

echo "Generating Go bindings for contract errors..."

# Check if abigen is installed
if ! command -v abigen &> /dev/null; then
    echo "Error: abigen is not installed. Install it with:"
    echo "  go install github.com/ethereum/go-ethereum/cmd/abigen@latest"
    exit 1
fi

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed"
    exit 1
fi

# Ensure contracts are built
if [ ! -d "$OUT_DIR" ]; then
    echo "Error: Contracts not built. Run 'make build' in service_contracts/ first"
    exit 1
fi

# Create pkg directory if it doesn't exist
mkdir -p "$PKG_DIR"

# Function to extract errors from ABI and generate Go bindings
generate_error_bindings() {
    local contract_name=$1
    local abi_path=$2
    local go_package=$3
    local go_type=$4

    echo "Processing $contract_name..."

    # Check if ABI file exists
    if [ ! -f "$abi_path" ]; then
        echo "Warning: ABI file not found: $abi_path"
        return 1
    fi

    # Extract only error definitions from ABI
    local errors_abi=$(jq '[.[] | select(.type == "error")]' "$abi_path")

    # Count errors
    local error_count=$(echo "$errors_abi" | jq 'length')

    if [ "$error_count" -eq 0 ]; then
        echo "  No errors found in $contract_name"
        return 0
    fi

    echo "  Found $error_count errors"

    # Create a minimal ABI JSON with just errors
    local temp_abi=$(mktemp)
    echo "$errors_abi" > "$temp_abi"

    # Generate Go bindings using abigen
    local output_file="$PKG_DIR/${go_package}_errors.go"

    abigen \
        --abi "$temp_abi" \
        --pkg fvmerrors \
        --type "$go_type" \
        --out "$output_file"

    rm "$temp_abi"

    echo "  Generated: $output_file"
    return 0
}

# Generate bindings for each contract's errors

# 1. FilecoinWarmStorageService errors (from Errors.sol)
generate_error_bindings \
    "FilecoinWarmStorageService Errors" \
    "$OUT_DIR/Errors.sol/Errors.json" \
    "warmstorage" \
    "WarmStorageErrors"

# 2. Payments errors
generate_error_bindings \
    "Payments Errors" \
    "$OUT_DIR/../lib/fws-payments/out/Errors.sol/Errors.json" \
    "payments" \
    "PaymentsErrors"

# 3. PDPVerifier errors
generate_error_bindings \
    "PDP Verifier Errors" \
    "$OUT_DIR/../lib/pdp/out/PDPVerifier.sol/PDPVerifier.json" \
    "pdp" \
    "PDPErrors"

echo ""
echo "Go bindings generated successfully in $PKG_DIR"
echo ""
echo "To use in your Go code:"
echo "  import \"github.com/storacha/filecoin-services/pkg/fvmerrors\""
echo ""
echo "Regenerate after contract changes with:"
echo "  ./tools/gen-error-bindings.sh"
