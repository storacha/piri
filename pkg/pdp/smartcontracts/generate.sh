#!/bin/bash
set -e

# macOS uses BSD sed while Ubuntu uses GNU sed.
# we need this script to work on both OSs.
sedi() {
  sed "$@" > tmpfile && mv tmpfile "${@: -1}"
}

# Check if abigen is available
if ! command -v abigen &> /dev/null; then
    echo "Error: abigen not found. Please install go-ethereum tools."
    echo "Run: go install github.com/ethereum/go-ethereum/cmd/abigen@latest"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "Error: jq not found. Please install jq"
    exit 1
fi

echo "=== Contract Generation Script ==="
echo "Building contracts and generating Go bindings..."

# Change to script directory
cd "$(dirname "$0")"

echo
echo "Step 1: Building PDP contracts..."
cd contracts/pdp
make build
cd -

echo
echo "Step 2: Building Filecoin services contracts..."
cd contracts/filecoin-services/service_contracts
make build
cd -

echo
echo "Step 3: Generating Go bindings..."

# Create clean bindings directory
rm -rf bindings
mkdir -p bindings

# First, extract ABIs from JSON artifacts
echo "Extracting ABIs from JSON artifacts..."

# Extract PDP contract ABIs
jq -r '.abi' contracts/pdp/out/PDPVerifier.sol/PDPVerifier.json > contracts/pdp/out/PDPVerifier.sol/PDPVerifier.abi
jq -r '.abi' contracts/pdp/out/IPDPProvingSchedule.sol/IPDPProvingSchedule.json > contracts/pdp/out/IPDPProvingSchedule.sol/IPDPProvingSchedule.abi 2>/dev/null || echo "IPDPProvingSchedule not found, skipping"

# Extract Filecoin services contract ABIs
jq -r '.abi' contracts/filecoin-services/service_contracts/out/FilecoinWarmStorageService.sol/FilecoinWarmStorageService.json > contracts/filecoin-services/service_contracts/out/FilecoinWarmStorageService.sol/FilecoinWarmStorageService.abi
jq -r '.abi' contracts/filecoin-services/service_contracts/out/ServiceProviderRegistry.sol/ServiceProviderRegistry.json > contracts/filecoin-services/service_contracts/out/ServiceProviderRegistry.sol/ServiceProviderRegistry.abi

# Generate a common types file first
echo "Creating common types file..."
cat > bindings/common_types.go << 'EOF'
// Code generated - DO NOT EDIT.
package bindings

import (
	"math/big"
)

// CidsCid is an auto generated low-level Go binding around an user-defined struct.
type CidsCid struct {
	Data []byte
}

// Common types used across contracts
type IPDPTypesProof struct {
	Leaf  [32]byte
	Proof [][32]byte
}

type IPDPTypesPieceIdAndOffset struct {
	PieceId *big.Int
	Offset  *big.Int
}
EOF

# Generate bindings from PDP contracts
echo "Generating PDPVerifier bindings..."
abigen --abi contracts/pdp/out/PDPVerifier.sol/PDPVerifier.abi \
       --pkg bindings \
       --type PDPVerifier \
       --out bindings/pdp_verifier_temp.go

# Remove duplicate type definitions
echo "Removing duplicate types from PDPVerifier..."
sedi '/^type CidsCid struct {$/,/^}$/d' bindings/pdp_verifier_temp.go
sedi '/^type IPDPTypesProof struct {$/,/^}$/d' bindings/pdp_verifier_temp.go
sedi '/^type IPDPTypesPieceIdAndOffset struct {$/,/^}$/d' bindings/pdp_verifier_temp.go
mv bindings/pdp_verifier_temp.go bindings/pdp_verifier.go

# Try to generate PDPProvingSchedule if it exists
if [ -f "contracts/pdp/out/IPDPProvingSchedule.sol/IPDPProvingSchedule.abi" ]; then
    echo "Generating PDPProvingSchedule bindings..."
    abigen --abi contracts/pdp/out/IPDPProvingSchedule.sol/IPDPProvingSchedule.abi \
           --pkg bindings \
           --type PDPProvingSchedule \
           --out bindings/pdp_proving_schedule.go
fi

# Generate bindings from Filecoin services
echo "Generating FilecoinWarmStorageService bindings..."
abigen --abi contracts/filecoin-services/service_contracts/out/FilecoinWarmStorageService.sol/FilecoinWarmStorageService.abi \
       --pkg bindings \
       --type FilecoinWarmStorageService \
       --out bindings/filecoin_warm_storage_service_temp.go

# Remove duplicate types
sedi '/^type CidsCid struct {$/,/^}$/d' bindings/filecoin_warm_storage_service_temp.go
mv bindings/filecoin_warm_storage_service_temp.go bindings/filecoin_warm_storage_service.go

echo "Generating ServiceProviderRegistry bindings..."
abigen --abi contracts/filecoin-services/service_contracts/out/ServiceProviderRegistry.sol/ServiceProviderRegistry.abi \
       --pkg bindings \
       --type ServiceProviderRegistry \
       --out bindings/service_provider_registry.go

echo
echo "✅ Contract generation complete!"
echo "Generated files in: bindings/"