#!/bin/bash
set -e

# Configuration - update these versions as needed
PDP_VERSION="${PDP_VERSION:-v2.1.0}"
#FILECOIN_SERVICES_VERSION="${FILECOIN_SERVICES_VERSION:-latest}"

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

if ! command -v curl &> /dev/null; then
    echo "Error: curl not found. Please install curl"
    exit 1
fi

echo "=== Contract Generation Script ==="
echo "Downloading ABIs from releases and generating Go bindings..."
echo "PDP version: $PDP_VERSION"
#echo "Filecoin Services version: $FILECOIN_SERVICES_VERSION"

# Change to script directory
cd "$(dirname "$0")"

# Create clean directories
rm -rf abis bindings
mkdir -p abis bindings

echo
echo "Step 1: Downloading PDP contract ABIs from FilOzone/pdp $PDP_VERSION..."
curl -fsSL "https://github.com/FilOzone/pdp/releases/download/$PDP_VERSION/PDPVerifier.abi.json" -o abis/PDPVerifier.abi.json
curl -fsSL "https://github.com/FilOzone/pdp/releases/download/$PDP_VERSION/IPDPProvingSchedule.abi.json" -o abis/IPDPProvingSchedule.abi.json

#echo
#echo "Step 2: Downloading Filecoin Services contract ABIs..."
#if [ "$FILECOIN_SERVICES_VERSION" = "latest" ]; then
    # Get the latest release tag
#    FILECOIN_SERVICES_VERSION=$(curl -fsSL https://api.github.com/repos/FilOzone/filecoin-services/releases/latest | jq -r '.tag_name')
#    if [ "$FILECOIN_SERVICES_VERSION" = "null" ] || [ -z "$FILECOIN_SERVICES_VERSION" ]; then
#        echo "Error: No releases found for FilOzone/filecoin-services"
#        echo "Please set FILECOIN_SERVICES_VERSION environment variable to a specific release tag"
#        exit 1
#    fi
#    echo "Found latest release: $FILECOIN_SERVICES_VERSION"
#fi

#curl -fsSL "https://github.com/FilOzone/filecoin-services/releases/download/$FILECOIN_SERVICES_VERSION/FilecoinWarmStorageService.abi.json" -o abis/FilecoinWarmStorageService.abi.json
#curl -fsSL "https://github.com/FilOzone/filecoin-services/releases/download/$FILECOIN_SERVICES_VERSION/ServiceProviderRegistry.abi.json" -o abis/ServiceProviderRegistry.abi.json

echo
echo "Step 2: Extracting ABIs from JSON artifacts..."
jq -r '.' abis/PDPVerifier.abi.json > abis/PDPVerifier.abi
jq -r '.' abis/IPDPProvingSchedule.abi.json > abis/IPDPProvingSchedule.abi
#jq -r '.' abis/FilecoinWarmStorageService.abi.json > abis/FilecoinWarmStorageService.abi
#jq -r '.' abis/ServiceProviderRegistry.abi.json > abis/ServiceProviderRegistry.abi

echo
echo "Step 3: Generating Go bindings..."

# Generate a common types file first, needed since contracts define these independently, without this we get duplicate types.
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
abigen --abi abis/PDPVerifier.abi \
       --pkg bindings \
       --type PDPVerifier \
       --out bindings/pdp_verifier_temp.go

# Remove duplicate type definitions
echo "Removing duplicate types from PDPVerifier..."
sedi '/^type CidsCid struct {$/,/^}$/d' bindings/pdp_verifier_temp.go
sedi '/^type IPDPTypesProof struct {$/,/^}$/d' bindings/pdp_verifier_temp.go
sedi '/^type IPDPTypesPieceIdAndOffset struct {$/,/^}$/d' bindings/pdp_verifier_temp.go
mv bindings/pdp_verifier_temp.go bindings/pdp_verifier.go

echo "Generating PDPProvingSchedule bindings..."
abigen --abi abis/IPDPProvingSchedule.abi \
       --pkg bindings \
       --type PDPProvingSchedule \
       --out bindings/pdp_proving_schedule.go

# Generate bindings from Filecoin services
#echo "Generating FilecoinWarmStorageService bindings..."
#abigen --abi abis/FilecoinWarmStorageService.abi \
#       --pkg bindings \
#       --type FilecoinWarmStorageService \
#       --out bindings/filecoin_warm_storage_service_temp.go

# Remove duplicate types
#sedi '/^type CidsCid struct {$/,/^}$/d' bindings/filecoin_warm_storage_service_temp.go
#mv bindings/filecoin_warm_storage_service_temp.go bindings/filecoin_warm_storage_service.go

#echo "Generating ServiceProviderRegistry bindings..."
#abigen --abi abis/ServiceProviderRegistry.abi \
#       --pkg bindings \
#       --type ServiceProviderRegistry \
#       --out bindings/service_provider_registry.go

echo
echo "✅ Contract generation complete!"
echo "Generated files in: bindings/"
echo "ABIs downloaded from:"
echo "  - FilOzone/pdp: $PDP_VERSION"
#echo "  - FilOzone/filecoin-services: $FILECOIN_SERVICES_VERSION"