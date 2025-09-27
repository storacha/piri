# Filecoin Services Deployment Guide for Calibration Network

This guide provides step-by-step instructions for deploying the Filecoin Services contracts on the Filecoin Calibration testnet from scratch.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Wallet Setup](#wallet-setup)
- [Repository Setup](#repository-setup)
- [Environment Configuration](#environment-configuration)
- [Deployment Process](#deployment-process)
- [Post-Deployment](#post-deployment)
- [Contract Interaction](#contract-interaction)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### 1. Install Required Tools

#### Foundry (includes forge and cast)
Foundry is a blazing fast, portable and modular toolkit for Ethereum application development.

```bash
# Install Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup
```

Verify installation:
```bash
forge --version
cast --version
```

#### jq (JSON processor)
jq is required for parsing JSON responses from the deployment scripts.

```bash
# Ubuntu/Debian
sudo apt-get update && sudo apt-get install jq

# macOS
brew install jq

# Other systems - download from https://jqlang.github.io/jq/download/
```

Verify installation (should be v1.7+):
```bash
jq --version
```

#### Git
Ensure you have Git installed with submodule support:
```bash
git --version
```

## Wallet Setup

### 1. Create a New Keystore

A keystore is an encrypted JSON file containing your private key. You'll need this to deploy contracts.

**Option A: Create a new wallet**
```bash
# This will prompt you for a password
cast wallet new -k keystore.json

# Save the displayed address - you'll need to fund it
```

**Option B: Import existing private key**
```bash
# Replace <YOUR_PRIVATE_KEY> with your actual private key (without 0x prefix)
cast wallet import -k keystore.json --private-key <YOUR_PRIVATE_KEY>
```

### 2. Get Your Wallet Address
```bash
# You'll be prompted for the password you set
cast wallet address --keystore keystore.json --password <PASSWORD>
```

### 3. Fund Your Wallet

You need testnet FIL to pay for gas fees:

1. Visit the Filecoin Calibration faucet (search for "Filecoin calibration faucet")
2. Enter your wallet address
3. Request at least 0.5 FIL (recommended for deploying all contracts)

Verify your balance:
```bash
export RPC_URL="https://api.calibration.node.glif.io/rpc/v1"
cast balance <YOUR_ADDRESS> --rpc-url $RPC_URL
```

## Repository Setup

### 1. Clone the Repository
```bash
git clone https://github.com/FilOzone/filecoin-services.git
cd filecoin-services
```

### 2. Navigate to Service Contracts Directory
```bash
cd service_contracts
```

### 3. Install Dependencies and Build
```bash
# Install git submodules and dependencies
make install

# Build all contracts
make build

# (Optional) Run tests to verify everything works
make test
```

## Environment Configuration

Create a deployment script or set these environment variables in your shell:

```bash
# Create a deployment environment file
cat > deploy-env.sh << 'EOF'
#!/bin/bash

# Network Configuration
export RPC_URL="https://api.calibration.node.glif.io/rpc/v1"

# Wallet Configuration (use absolute paths)
export KEYSTORE="$(pwd)/keystore.json"
export PASSWORD="your-keystore-password"

# Contract Parameters
export CHALLENGE_FINALITY="900"          # ~15 minutes on calibnet (30s blocks)
export MAX_PROVING_PERIOD="30"           # 30 epochs (15 minutes)
export CHALLENGE_WINDOW_SIZE="15"        # 15 epochs

# Service Metadata (customize these)
export SERVICE_NAME="My Warm Storage Service"
export SERVICE_DESCRIPTION="A decentralized storage service for warm data on Filecoin"

# FilCDN Addresses (optional - defaults will be used if not set)
# export FILCDN_CONTROLLER_ADDRESS="0x5f7E5E2A756430EdeE781FF6e6F7954254Ef629A"
# export FILCDN_BENEFICIARY_ADDRESS="0x1D60d2F5960Af6341e842C539985FA297E10d6eA"

# Session Key Registry (optional - will deploy new one if not set)
# export SESSION_KEY_REGISTRY_ADDRESS="0x..."

echo "Environment configured for deployment"
EOF

# Make the script executable
chmod +x deploy-env.sh
```

## Deployment Process

### 1. Load Environment Variables
```bash
# From the service_contracts directory
source ./deploy-env.sh
```

### 2. Verify Configuration
```bash
# Check your wallet balance
cast balance $(cast wallet address --keystore "$KEYSTORE" --password "$PASSWORD") --rpc-url "$RPC_URL"

# Verify you're on the correct network (should return 314159)
cast chain-id --rpc-url "$RPC_URL"
```

### 3. Run the Deployment Script
```bash
# Ensure you're in the service_contracts directory
./tools/deploy-all-warm-storage-calibnet.sh
```

The script will deploy the following contracts in order:
1. **SessionKeyRegistry** (if not provided via environment variable)
2. **PDPVerifier** (implementation + proxy)
3. **Payments** contract
4. **ServiceProviderRegistry** (implementation + proxy)
5. **FilecoinWarmStorageService** (implementation + proxy)
6. **FilecoinWarmStorageServiceStateView**
7. Configure the view contract on the main service

### 4. Save Deployment Output

The script will output all deployed contract addresses. Save these addresses! Example output:

```
# DEPLOYMENT SUMMARY
PDPVerifier Implementation: 0x...
PDPVerifier Proxy: 0x...
Payments Contract: 0x...
ServiceProviderRegistry Implementation: 0x...
ServiceProviderRegistry Proxy: 0x...
FilecoinWarmStorageService Implementation: 0x...
FilecoinWarmStorageService Proxy: 0x...  <-- Main contract to interact with
FilecoinWarmStorageServiceStateView: 0x...
```

## Post-Deployment

### 1. Verify Deployment

Check that your main contract is deployed:
```bash
# Replace with your FilecoinWarmStorageService proxy address
WARM_STORAGE_PROXY="0x..."
cast code $WARM_STORAGE_PROXY --rpc-url $RPC_URL
```

### 2. Check Contract State

Verify the contract was initialized correctly:
```bash
# Get service name
cast call $WARM_STORAGE_PROXY "serviceName()" --rpc-url $RPC_URL | cast --to-ascii

# Get service description
cast call $WARM_STORAGE_PROXY "serviceDescription()" --rpc-url $RPC_URL | cast --to-ascii

# Get max proving period
cast call $WARM_STORAGE_PROXY "maxProvingPeriod()" --rpc-url $RPC_URL
```

## Contract Interaction

### Using Cast Commands

Example interactions with your deployed contract:

```bash
# Check if an address has admin role
cast call $WARM_STORAGE_PROXY "hasRole(bytes32,address)" \
  $(cast keccak "DEFAULT_ADMIN_ROLE") \
  <YOUR_ADDRESS> \
  --rpc-url $RPC_URL

# Get challenge window size
cast call $WARM_STORAGE_PROXY "challengeWindowSize()" --rpc-url $RPC_URL
```

### Creating Data Sets

To create a data set, you'll need to:
1. Have USDFC tokens for payment
2. Prepare the transaction parameters
3. Call the appropriate function on the contract

### Using View Contract

The deployed view contract provides read-only access to contract state:
```bash
VIEW_CONTRACT="0x..."  # Your FilecoinWarmStorageServiceStateView address
# Query data set information, challenge states, etc.
```

## Troubleshooting

### Common Issues

1. **"RPC_URL is not set"**
   - Ensure you've sourced the deploy-env.sh file
   - Or export the RPC_URL manually

2. **"Failed to detect chain ID"**
   - Check your internet connection
   - Verify the RPC URL is correct

3. **"Insufficient funds"**
   - Check your wallet balance
   - Request more testnet FIL from the faucet

4. **"Challenge finality validation failed"**
   - Ensure MAX_PROVING_PERIOD is large enough for your CHALLENGE_FINALITY
   - MIN_REQUIRED = CHALLENGE_FINALITY + (CHALLENGE_WINDOW_SIZE / 2)

5. **Transaction fails**
   - Check gas prices on the network
   - Verify you have enough FIL for gas
   - Check for any reverted transactions for error messages

### Getting Help

- Check the deployment script output for specific error messages
- Verify all environment variables are set correctly
- Ensure you're in the correct directory (service_contracts)
- Join the Filecoin community channels for support

## Security Notes

- **Never share your private key or keystore password**
- **Keep your keystore.json file secure**
- **Use different wallets for testnet and mainnet**
- **These contracts are not audited - use at your own risk**

## Next Steps

After successful deployment:
1. Test basic contract functionality
2. Set up monitoring for your contracts
3. Integrate with your application
4. Consider upgrading contracts as needed (UUPS pattern)