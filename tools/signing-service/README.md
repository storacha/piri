# Signing Service Integration

This document describes the signing service integration for authenticated PDP operations with the FilecoinWarmStorageService contract.

## Overview

The signing service provides EIP-712 signature authentication for PDP operations. It supports two modes of operation:

1. **Remote Signing Service**: Connect to an external signing service via HTTP
2. **In-Process Signing**: Sign operations directly using a configured private key

## Configuration

Add the following configuration to your `config.toml` file:

```toml
[pdpservice.signingservice]
# Enable signing service integration
enabled = true

# Chain ID for EIP-712 signatures (314159 for Filecoin Calibration testnet)
chain_id = 314159

# Address of the payer (account that pays for storage)
payer_address = "0x1111111111111111111111111111111111111111"

# Address of the FilecoinWarmStorageService contract
service_contract_address = "0x2222222222222222222222222222222222222222"

# Option 1: Use HTTP client for remote signing service
# endpoint = "http://localhost:8080"

# Option 2: Use in-process signer with private key
private_key = "0xYOUR_PRIVATE_KEY_HERE"
```

## Security Considerations

- **Never commit private keys to version control**
- Use environment variables for sensitive configuration in production
- Consider using a hardware security module (HSM) or key management service for production deployments
- The remote signing service option is recommended for production use

## Supported Operations

The signing service enables authenticated operations for:

1. **CreateDataSet**: Create new datasets with signed metadata
2. **AddPieces**: Add pieces to existing datasets
3. **SchedulePieceRemovals**: Schedule pieces for removal
4. **DeleteDataSet**: Delete entire datasets

## How It Works

1. When creating a proof set (dataset), the service:
   - Queries the view contract for the next client dataset ID
   - Creates an EIP-712 signature for the operation
   - Encodes the signature and metadata into extraData
   - Sends the transaction to the PDPVerifier contract
   - The FilecoinWarmStorageService validates the signature on-chain

2. The signature includes:
   - Client dataset ID (preventing replay attacks)
   - Payee address (recipient of payments)
   - Metadata key-value pairs
   - Domain separator (chain ID and contract address)

## View Contract Integration

The service automatically integrates with the FilecoinWarmStorageServiceStateView contract to:
- Query the next client dataset ID for signing
- Retrieve dataset information
- Check provider approval status
- Access metadata for datasets and pieces

## Running the Signing Service

### Standalone Signing Service

To run a standalone signing service:

```bash
cd tools/signing-service
go run main.go --private-key=0xYOUR_PRIVATE_KEY --chain-id=314159 --contract=0xCONTRACT_ADDRESS
```

### Integrated Mode

When using in-process signing, the service is automatically initialized based on configuration.

## Testing

To test the signing service integration:

1. Configure the service with test credentials
2. Create a dataset using the API:
   ```bash
   curl -X POST http://localhost:8080/api/v1/proofset/create \
     -H "Content-Type: application/json" \
     -d '{
       "recordKeeper": "0xYOUR_PAYEE_ADDRESS",
       "extraData": "key1=value1,key2=value2"
     }'
   ```
3. Verify the transaction includes the signed extraData
4. Check the on-chain validation succeeded

## Troubleshooting

Common issues and solutions:

1. **"Failed to get next client dataset ID"**: Ensure the view contract address is correctly configured
2. **"Invalid signature"**: Verify the chain ID and contract address match the on-chain deployment
3. **"Failed to encode extraData"**: Check that metadata follows the key=value format
4. **"Signing service not configured"**: Enable the signing service in configuration

## Related Components

- `/tools/signing-service`: Standalone signing service implementation
- `/tools/service-operator`: CLI tool for service operations
- `/pkg/pdp/smartcontracts`: Smart contract bindings and helpers
- `/pkg/pdp/service`: Core PDP service implementation