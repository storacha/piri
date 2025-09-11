#!/bin/bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1" >&2
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

# Ensure we're in the project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

log "Starting contract generation from ${PROJECT_ROOT}..."

# Check required tools and determine execution method
FORGE_CMD=""
ABIGEN_CMD=""
USE_DOCKER=false

# Check if forge is available natively
if command -v forge >/dev/null 2>&1; then
    FORGE_CMD="forge"
    log "Found native forge installation"
elif command -v docker >/dev/null 2>&1; then
    FORGE_CMD="docker run --rm -v $(pwd)/contracts/pdp:/workspace -w /workspace ghcr.io/foundry-rs/foundry:latest forge"
    USE_DOCKER=true
    log "Using Docker for forge (foundry)"
else
    error "Neither forge nor Docker is available. Please install Foundry or Docker."
    exit 1
fi

# Check for abigen
if command -v abigen >/dev/null 2>&1; then
    ABIGEN_CMD="abigen"
    log "Found abigen in PATH"
elif [ -f "$(go env GOPATH)/bin/abigen" ]; then
    ABIGEN_CMD="$(go env GOPATH)/bin/abigen"
    log "Found abigen in GOPATH/bin"
else
    error "abigen not found. Run 'make tools' to install it."
    exit 1
fi

# Check for jq
if ! command -v jq >/dev/null 2>&1; then
    error "jq not found. Please install jq to parse JSON files."
    log "On macOS: brew install jq"
    log "On Ubuntu: apt-get install jq"
    exit 1
fi

# Ensure submodule is present and up to date
if [ ! -d "contracts/pdp" ]; then
    error "contracts/pdp submodule not found. Run 'git submodule update --init'"
    exit 1
fi

if [ ! -d "contracts/pdp/src" ]; then
    error "contracts/pdp/src not found. The submodule may not be properly initialized."
    exit 1
fi

# Build contracts using Forge
log "Building contracts with Forge..."
if [ "$USE_DOCKER" = true ]; then
    # Docker approach - mount the contracts directory
    log "Running forge build via Docker..."
    eval "$FORGE_CMD build --silent" || {
        error "Forge build failed via Docker"
        exit 1
    }
else
    # Native approach
    cd contracts/pdp
    eval "$FORGE_CMD build --silent" || {
        error "Forge build failed"
        exit 1
    }
    cd ../..
fi

# Verify build artifacts exist
if [ ! -d "contracts/pdp/out" ]; then
    error "Build artifacts directory (out/) not found after forge build"
    exit 1
fi

# Contract mapping: ContractName -> SolidityFile -> ABI/BIN paths -> Output file
log "Generating Go bindings..."

# Create temp directory for ABI files
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Process PDPVerifier contract
CONTRACT_NAME="PDPVerifier"
SOL_FILE="PDPVerifier.sol"
JSON_FILE="contracts/pdp/out/${SOL_FILE}/${CONTRACT_NAME}.json"
BIN_FILE="contracts/pdp/out/${SOL_FILE}/${CONTRACT_NAME}.bin"
OUTPUT_FILE="pkg/pdp/service/contract/internal/pdp_verifier.go"

log "Processing ${CONTRACT_NAME}..."

if [ ! -f "$JSON_FILE" ]; then
    error "JSON file not found: $JSON_FILE"
    log "Available files in out/${SOL_FILE}/:"
    ls -la "contracts/pdp/out/${SOL_FILE}/" 2>/dev/null || echo "Directory does not exist"
    exit 1
fi

# Extract ABI from JSON (Foundry wraps it in a JSON object)
jq '.abi' "$JSON_FILE" > "$TEMP_DIR/${CONTRACT_NAME}.abi"

# Generate Go binding
if [ -f "$BIN_FILE" ]; then
    log "  Generating with bytecode..."
    $ABIGEN_CMD --abi "$TEMP_DIR/${CONTRACT_NAME}.abi" --bin "$BIN_FILE" \
               --pkg internal --type "$CONTRACT_NAME" --out "$OUTPUT_FILE"
else
    log "  Generating without bytecode (interface only)..."
    $ABIGEN_CMD --abi "$TEMP_DIR/${CONTRACT_NAME}.abi" \
               --pkg internal --type "$CONTRACT_NAME" --out "$OUTPUT_FILE"
fi
success "Generated ${OUTPUT_FILE}"

# Process IPDPProvingSchedule interface
CONTRACT_NAME="IPDPProvingSchedule"
SOL_FILE="IPDPProvingSchedule.sol"
JSON_FILE="contracts/pdp/out/${SOL_FILE}/${CONTRACT_NAME}.json"
BIN_FILE="contracts/pdp/out/${SOL_FILE}/${CONTRACT_NAME}.bin"
OUTPUT_FILE="pkg/pdp/service/contract/internal/pdp_proving_schedule.go"

log "Processing ${CONTRACT_NAME}..."

if [ ! -f "$JSON_FILE" ]; then
    error "JSON file not found: $JSON_FILE"
    log "Available files in out/${SOL_FILE}/:"
    ls -la "contracts/pdp/out/${SOL_FILE}/" 2>/dev/null || echo "Directory does not exist"
    exit 1
fi

# Extract ABI from JSON
jq '.abi' "$JSON_FILE" > "$TEMP_DIR/${CONTRACT_NAME}.abi"

# Generate Go binding (interfaces typically don't have bytecode)
if [ -f "$BIN_FILE" ] && [ -s "$BIN_FILE" ]; then
    log "  Generating with bytecode..."
    $ABIGEN_CMD --abi "$TEMP_DIR/${CONTRACT_NAME}.abi" --bin "$BIN_FILE" \
               --pkg internal --type "$CONTRACT_NAME" --out "$OUTPUT_FILE"
else
    log "  Generating without bytecode (interface only)..."
    $ABIGEN_CMD --abi "$TEMP_DIR/${CONTRACT_NAME}.abi" \
               --pkg internal --type "$CONTRACT_NAME" --out "$OUTPUT_FILE"
fi
success "Generated ${OUTPUT_FILE}"

# Update VERSION file
log "Updating VERSION file..."
cd contracts/pdp
COMMIT_SHA=$(git rev-parse HEAD)
cd ../..

cat > pkg/pdp/service/contract/VERSION << EOF
PDP_CONTRACT_VERSION=${COMMIT_SHA}
PDP_CONTRACT_COMMIT=${COMMIT_SHA}
GENERATED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
FORGE_METHOD=$([ "$USE_DOCKER" = true ] && echo "docker" || echo "native")
EOF

success "Contract generation completed successfully!"
echo ""
log "Summary:"
echo "  - Generated bindings for: PDPVerifier, IPDPProvingSchedule"
echo "  - Submodule commit: ${COMMIT_SHA}"
echo "  - Build method: $([ "$USE_DOCKER" = true ] && echo "Docker" || echo "Native")"
echo "  - VERSION file updated"

# Verify generated files exist and are not empty
for file in "pkg/pdp/service/contract/internal/pdp_verifier.go" "pkg/pdp/service/contract/internal/pdp_proving_schedule.go"; do
    if [ ! -f "$file" ] || [ ! -s "$file" ]; then
        error "Generated file is missing or empty: $file"
        exit 1
    fi
done

success "All generated files verified successfully!"