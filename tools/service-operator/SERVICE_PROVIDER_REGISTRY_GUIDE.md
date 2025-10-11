# ServiceProviderRegistry Usage Guide

This guide explains how to register as a service provider, manage your provider information, and query the registry on the Filecoin network.

## Table of Contents
- [Overview](#overview)
- [Key Concepts](#key-concepts)
- [Registration Process](#registration-process)
- [Managing Provider Information](#managing-provider-information)
- [Product Management](#product-management)
- [Querying the Registry](#querying-the-registry)
- [Code Examples](#code-examples)
- [Important Notes](#important-notes)

## Overview

The ServiceProviderRegistry is a **self-service registry** that allows service providers to register and manage their offerings on the Filecoin network. Unlike traditional registries, there is **no approval process** - providers register themselves by paying a 5 FIL fee that is permanently burned.

### Key Features
- **Decentralized**: No central authority approval required
- **Economic Security**: 5 FIL registration fee deters spam
- **Self-Management**: Providers control their own information
- **Immutable Payee**: Payment address cannot be changed after registration
- **Product Support**: Currently supports PDP (Proof of Data Possession) services

## Key Concepts

### Provider ID
Each registered provider receives a unique numeric ID that identifies them in the system.

### Service Provider vs Payee
- **Service Provider**: The address that registers and manages the provider account
- **Payee**: The address that receives payments (set during registration, cannot be changed)

### Products
Providers can offer different product types. Currently only PDP is supported:
- **PDP (Proof of Data Possession)**: Storage verification services

### Capabilities
Key-value pairs that describe additional features or requirements of a product (max 10 pairs per product).

## Registration Process

### Prerequisites
1. Have at least 5 FIL in your wallet for the registration fee
2. Deployed ServiceProviderRegistry contract address
3. Decide on your payee address (cannot be changed later!)

### Registration Steps

1. **Prepare Registration Data**
```javascript
// Example PDP offering structure
const pdpOffering = {
  serviceURL: "https://api.myprovider.com/pdp",           // Your API endpoint
  minPieceSizeInBytes: 1048576,                          // 1 MB minimum
  maxPieceSizeInBytes: 34359738368,                      // 32 GB maximum
  ipniPiece: false,                                      // IPNI piece support
  ipniIpfs: false,                                       // IPNI IPFS support
  storagePricePerTibPerMonth: "1000000000000000000",    // Price in attoFIL
  minProvingPeriodInEpochs: 30,                          // Minimum proving period
  location: "US-East",                                    // Service location
  paymentTokenAddress: "0x0000000000000000000000000000000000000000" // Payment token
};

// Encode the offering
const productData = ethers.utils.defaultAbiCoder.encode(
  ["tuple(string,uint256,uint256,bool,bool,uint256,uint256,string,address)"],
  [pdpOffering]
);
```

2. **Set Capabilities (Optional)**
```javascript
// Example capabilities
const capabilityKeys = ["region", "redundancy", "bandwidth"];
const capabilityValues = ["us-east-1", "3x", "10Gbps"];
```

3. **Register with Cast**
```bash
# Set your registry address
REGISTRY_ADDRESS="0x..." # Your deployed ServiceProviderRegistry proxy address

# Register as a provider (requires exactly 5 FIL)
cast send $REGISTRY_ADDRESS \
  "registerProvider(address,string,string,uint8,bytes,string[],string[])" \
  $PAYEE_ADDRESS \
  "My Storage Provider" \
  "High-performance storage with 99.99% uptime" \
  0 \
  $PRODUCT_DATA \
  '["region","redundancy","bandwidth"]' \
  '["us-east-1","3x","10Gbps"]' \
  --value 5ether \
  --keystore $KEYSTORE \
  --password $PASSWORD \
  --rpc-url $RPC_URL
```

### What Happens During Registration
1. Contract validates all inputs (name length, description length, etc.)
2. Checks that the sender isn't already registered
3. Verifies exactly 5 FIL was sent
4. Assigns a unique provider ID
5. Stores provider information
6. Adds the initial product (PDP)
7. Burns the 5 FIL registration fee
8. Emits `ProviderRegistered` and `ProductAdded` events

### Getting Your Provider ID
After registration, query your provider ID:
```bash
# Get provider ID by address
cast call $REGISTRY_ADDRESS \
  "getProviderIdByAddress(address)" \
  $YOUR_ADDRESS \
  --rpc-url $RPC_URL
```

## Managing Provider Information

### Update Provider Info
You can update your name and description at any time:

```bash
# Update provider information
cast send $REGISTRY_ADDRESS \
  "updateProviderInfo(string,string)" \
  "Updated Provider Name" \
  "New description with more details about our services" \
  --keystore $KEYSTORE \
  --password $PASSWORD \
  --rpc-url $RPC_URL
```

**Constraints:**
- Name: Maximum 128 characters
- Description: Maximum 256 characters
- Only the registered service provider address can update

### Deactivate Provider
While there's no explicit deactivation function shown, providers can remove all their products to effectively become inactive.

## Product Management

### Add a New Product
If you registered without a product or want to add another product type:

```bash
# Add a PDP product
cast send $REGISTRY_ADDRESS \
  "addProduct(uint8,bytes,string[],string[])" \
  0 \
  $PRODUCT_DATA \
  '["capability1","capability2"]' \
  '["value1","value2"]' \
  --keystore $KEYSTORE \
  --password $PASSWORD \
  --rpc-url $RPC_URL
```

### Update Existing Product
Modify your product configuration or capabilities:

```bash
# Update product configuration
cast send $REGISTRY_ADDRESS \
  "updateProduct(uint8,bytes,string[],string[])" \
  0 \
  $NEW_PRODUCT_DATA \
  '["updated-capability"]' \
  '["new-value"]' \
  --keystore $KEYSTORE \
  --password $PASSWORD \
  --rpc-url $RPC_URL
```

### Remove a Product
Remove a product offering:

```bash
# Remove PDP product (productType = 0)
cast send $REGISTRY_ADDRESS \
  "removeProduct(uint8)" \
  0 \
  --keystore $KEYSTORE \
  --password $PASSWORD \
  --rpc-url $RPC_URL
```

## Querying the Registry

### Get Provider Information

```bash
# Get provider info by ID
cast call $REGISTRY_ADDRESS \
  "getProvider(uint256)" \
  $PROVIDER_ID \
  --rpc-url $RPC_URL

# Get provider info by address
cast call $REGISTRY_ADDRESS \
  "getProviderByAddress(address)" \
  $PROVIDER_ADDRESS \
  --rpc-url $RPC_URL
```

### Get Product Information

```bash
# Get specific product for a provider
# productType: 0 = PDP
cast call $REGISTRY_ADDRESS \
  "getProduct(uint256,uint8)" \
  $PROVIDER_ID \
  0 \
  --rpc-url $RPC_URL

# Get PDP service details (convenience function)
cast call $REGISTRY_ADDRESS \
  "getPDPService(uint256)" \
  $PROVIDER_ID \
  --rpc-url $RPC_URL
```

### Query Product Capabilities

```bash
# Get single capability value
cast call $REGISTRY_ADDRESS \
  "getProductCapability(uint256,uint8,string)" \
  $PROVIDER_ID \
  0 \
  "region" \
  --rpc-url $RPC_URL

# Get multiple capability values
cast call $REGISTRY_ADDRESS \
  "getProductCapabilities(uint256,uint8,string[])" \
  $PROVIDER_ID \
  0 \
  '["region","redundancy","bandwidth"]' \
  --rpc-url $RPC_URL
```

### List Providers

```bash
# Get all active providers (paginated)
# Parameters: offset, limit (max 100)
cast call $REGISTRY_ADDRESS \
  "getAllActiveProviders(uint256,uint256)" \
  0 \
  50 \
  --rpc-url $RPC_URL

# Get providers by product type
# Parameters: productType, offset, limit
cast call $REGISTRY_ADDRESS \
  "getProvidersByProductType(uint8,uint256,uint256)" \
  0 \
  0 \
  20 \
  --rpc-url $RPC_URL

# Get only active providers by product type
cast call $REGISTRY_ADDRESS \
  "getActiveProvidersByProductType(uint8,uint256,uint256)" \
  0 \
  0 \
  20 \
  --rpc-url $RPC_URL
```

### Check Registration Status

```bash
# Check if address is registered
cast call $REGISTRY_ADDRESS \
  "isRegisteredProvider(address)" \
  $ADDRESS_TO_CHECK \
  --rpc-url $RPC_URL

# Check if provider is active
cast call $REGISTRY_ADDRESS \
  "isProviderActive(uint256)" \
  $PROVIDER_ID \
  --rpc-url $RPC_URL

# Get total provider count
cast call $REGISTRY_ADDRESS \
  "getProviderCount()" \
  --rpc-url $RPC_URL
```

## Code Examples

### JavaScript/ethers.js Registration Example

```javascript
const { ethers } = require('ethers');

async function registerAsProvider(signer, registryAddress, payeeAddress) {
  // Registry ABI (minimum needed for registration)
  const registryABI = [
    "function registerProvider(address payee, string name, string description, uint8 productType, bytes productData, string[] capabilityKeys, string[] capabilityValues) payable returns (uint256)"
  ];
  
  const registry = new ethers.Contract(registryAddress, registryABI, signer);
  
  // Prepare PDP offering
  const pdpOffering = {
    serviceURL: "https://api.example.com/pdp",
    minPieceSizeInBytes: ethers.BigNumber.from("1048576"), // 1 MB
    maxPieceSizeInBytes: ethers.BigNumber.from("34359738368"), // 32 GB
    ipniPiece: false,
    ipniIpfs: false,
    storagePricePerTibPerMonth: ethers.utils.parseEther("0.01"),
    minProvingPeriodInEpochs: 30,
    location: "US-East",
    paymentTokenAddress: ethers.constants.AddressZero
  };
  
  // Encode product data
  const productData = ethers.utils.defaultAbiCoder.encode(
    ["tuple(string,uint256,uint256,bool,bool,uint256,uint256,string,address)"],
    [pdpOffering]
  );
  
  // Set capabilities
  const capabilityKeys = ["tier", "sla", "support"];
  const capabilityValues = ["premium", "99.99%", "24/7"];
  
  // Register with 5 FIL fee
  const tx = await registry.registerProvider(
    payeeAddress,
    "Example Storage Provider",
    "Enterprise-grade storage solutions",
    0, // ProductType.PDP
    productData,
    capabilityKeys,
    capabilityValues,
    { value: ethers.utils.parseEther("5") }
  );
  
  const receipt = await tx.wait();
  
  // Extract provider ID from events
  const event = receipt.events.find(e => e.event === 'ProviderRegistered');
  const providerId = event.args.providerId;
  
  console.log(`Registered with Provider ID: ${providerId}`);
  return providerId;
}
```

### Query Provider Example

```javascript
async function queryProvider(provider, registryAddress, providerId) {
  const registryABI = [
    "function getProvider(uint256) view returns (tuple(address serviceProvider, address payee, string name, string description, bool isActive, uint256 providerId))",
    "function getPDPService(uint256) view returns (tuple(string serviceURL, uint256 minPieceSizeInBytes, uint256 maxPieceSizeInBytes, bool ipniPiece, bool ipniIpfs, uint256 storagePricePerTibPerMonth, uint256 minProvingPeriodInEpochs, string location, address paymentTokenAddress))"
  ];
  
  const registry = new ethers.Contract(registryAddress, registryABI, provider);
  
  // Get provider info
  const providerInfo = await registry.getProvider(providerId);
  console.log("Provider Info:", {
    serviceProvider: providerInfo.serviceProvider,
    payee: providerInfo.payee,
    name: providerInfo.name,
    description: providerInfo.description,
    isActive: providerInfo.isActive
  });
  
  // Get PDP service details
  const pdpService = await registry.getPDPService(providerId);
  console.log("PDP Service:", {
    serviceURL: pdpService.serviceURL,
    minSize: ethers.utils.formatUnits(pdpService.minPieceSizeInBytes, 0),
    maxSize: ethers.utils.formatUnits(pdpService.maxPieceSizeInBytes, 0),
    pricePerTiB: ethers.utils.formatEther(pdpService.storagePricePerTibPerMonth),
    location: pdpService.location
  });
}
```

## Important Notes

### Economic Model
- **Registration Fee**: 5 FIL (burned, not refundable)
- **No Recurring Fees**: Once registered, no additional registry fees
- **Self-Service**: No approval process or waiting period

### Security Considerations
- **Payee Address**: Choose carefully - it cannot be changed after registration
- **Service URL**: Ensure your service endpoint is secure and reliable
- **Capabilities**: Use standard keys for better discoverability

### Limitations
- **One Registration per Address**: Each address can only register once
- **Product Types**: Currently only PDP is supported
- **Capability Limits**: Maximum 10 key-value pairs per product
- **String Length Limits**:
  - Name: 128 characters
  - Description: 256 characters
  - Service URL: 256 characters
  - Location: 128 characters
  - Capability keys: 32 characters
  - Capability values: 128 characters

### Best Practices
1. **Test on Calibnet First**: Always test your registration on calibnet before mainnet
2. **Backup Provider ID**: Save your provider ID after registration
3. **Monitor Events**: Watch for registry events to track changes
4. **Update Regularly**: Keep your service URL and capabilities current
5. **Plan Payee Address**: Consider using a multisig or secure wallet for payee

### Contract Upgradeability
The ServiceProviderRegistry uses the UUPS (Universal Upgradeable Proxy Standard) pattern, meaning the contract logic can be upgraded while preserving state and addresses.