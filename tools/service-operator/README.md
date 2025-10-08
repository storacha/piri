# Service Operator CLI

A command-line tool for managing FilecoinWarmStorageService smart contracts.

## Installation

```bash
cd tools/service-operator
go build -o service-operator
```

## Configuration

The tool can be configured using command-line flags, environment variables, or a configuration file.

### Environment Variables

- `SERVICE_OPERATOR_RPC_URL` - Ethereum RPC endpoint URL
- `SERVICE_OPERATOR_CONTRACT_ADDRESS` - FilecoinWarmStorageService contract address
- `SERVICE_OPERATOR_PAYMENTS_ADDRESS` - Payments contract address
- `SERVICE_OPERATOR_TOKEN_ADDRESS` - ERC20 token contract address (must support EIP-2612)
- `SERVICE_OPERATOR_PRIVATE_KEY` - Path to private key file
- `SERVICE_OPERATOR_KEYSTORE` - Path to keystore file (alternative to private key)
- `SERVICE_OPERATOR_KEYSTORE_PASSWORD` - Keystore password
- `SERVICE_OPERATOR_NETWORK` - Network preset (calibration or mainnet)

### Network Presets

The `--network` flag provides convenient presets for known networks:

- `calibration` - Filecoin Calibration testnet
- `mainnet` - Filecoin Mainnet (not yet supported)

When using a network preset, the RPC URL and contract address are automatically configured. You can override these with explicit flags if needed.

## Commands

### List Providers

List service providers registered in the ServiceProviderRegistry.

```bash
service-operator provider list [flags]
```

**Flags:**
- `--limit <number>` - Maximum number of providers to display (default: 50)
- `--offset <number>` - Starting offset for pagination (default: 0)
- `--show-inactive` - Include inactive providers (default: only active)
- `--format <table|json>` - Output format (default: table)

**Note:** This is a read-only operation and does not require authentication. Only the RPC URL is needed.

#### Examples

List providers on calibration network:
```bash
service-operator provider list --network calibration
```

List with pagination:
```bash
service-operator provider list --network calibration --limit 20 --offset 40
```

Include inactive providers:
```bash
service-operator provider list --network calibration --show-inactive
```

JSON output for scripting:
```bash
service-operator provider list --network calibration --format json
```

Using explicit RPC URL:
```bash
service-operator provider list --rpc-url https://api.calibration.node.glif.io/rpc/v1
```

### Provider Approval

Approve a provider to create datasets in the FilecoinWarmStorageService.

```bash
service-operator provider approve <provider-id>
```

**Requirements:**
- The provider must already be registered in the ServiceProviderRegistry
- You must be the contract owner
- You must have enough FIL for gas fees

#### Examples

Using network preset (recommended):
```bash
export SERVICE_OPERATOR_PRIVATE_KEY="./owner-key.hex"
service-operator provider approve 123 --network calibration
```

Using environment variables:
```bash
export SERVICE_OPERATOR_RPC_URL="https://api.calibration.node.glif.io/rpc/v1"
export SERVICE_OPERATOR_CONTRACT_ADDRESS="0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91"
export SERVICE_OPERATOR_PRIVATE_KEY="./owner-key.hex"

service-operator provider approve 123
```

Using command-line flags:
```bash
service-operator provider approve 123 \
  --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
  --contract-address 0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91 \
  --private-key ./owner-key.hex
```

Using keystore instead of private key:
```bash
service-operator provider approve 123 \
  --network calibration \
  --keystore ./owner.keystore \
  --keystore-password mypassword
```

## Payments Commands

The payments commands manage interactions with the Payments contract, including calculating allowances, depositing tokens, and approving operators.

### Workflow Overview

There are three main workflows for using the Payments contract:

**Option 1: Deposit and Approve Together (Single Transaction)**
```bash
# Calculate allowances → Deposit + Approve in one step
service-operator payments calculate --size 1TiB
service-operator payments approve-service --deposit --amount 10000000 \
  --rate-allowance 57 --lockup-allowance 1641600 --max-lockup-period 86400
```

**Option 2: Deposit First, Then Approve Separately**
```bash
# Calculate allowances → Deposit → Approve
service-operator payments calculate --size 1TiB
service-operator payments deposit --amount 10000000
service-operator payments approve-service \
  --rate-allowance 57 --lockup-allowance 1641600 --max-lockup-period 86400
```

**Option 3: Approve First, Deposit Later**
```bash
# Calculate allowances → Approve → Deposit
service-operator payments calculate --size 1TiB
service-operator payments approve-service \
  --rate-allowance 57 --lockup-allowance 1641600 --max-lockup-period 86400
service-operator payments deposit --amount 10000000
```

All three workflows achieve the same result: funds in the Payments contract with the FilecoinWarmStorageService approved as an operator.

---

### Calculate Allowances

Calculate the rate allowance, lockup allowance, and max lockup period values needed for approving the service operator.

```bash
service-operator payments calculate --size <size> [flags]
```

**Required Flags:**
- `--size <size>` - Dataset size (e.g., 1TiB, 500GiB, 2.5TiB)

**Optional Flags:**
- `--lockup-days <days>` - Lockup period in days (default: 10)
- `--max-lockup-period-days <days>` - Maximum lockup period in days (default: 30)
- `--format <format>` - Output format: `human`, `shell`, or `flags` (default: human)

#### Examples

Calculate for 1 TiB with default 10 day lockup:

```bash
service-operator payments calculate --size 1TiB
```

Output:
```
Operator Approval Allowance Calculation
========================================

Input Parameters:
  Dataset size:           1.0 TiB (1099511627776 bytes)
  Lockup period:          10 days (28800 epochs)
  Max lockup period:      30 days (86400 epochs)
  Storage price:          $5.00 USD per TiB/month

Calculated Allowances:
  Rate allowance:         57 base units/epoch ($0.000057 per epoch)
  Lockup allowance:       1641600 base units ($1.64 for 10 days)
  Max lockup period:      86400 epochs (30 days)

Usage with approve-service:
  Copy these exact base unit values to the command:

  service-operator payments approve-service \
    --rate-allowance 57 \
    --lockup-allowance 1641600 \
    --max-lockup-period 86400
```

Shell-friendly output for scripting:

```bash
# Export as environment variables
eval $(service-operator payments calculate --size 1TiB --format shell)

# Use in commands
service-operator payments approve-service \
  --rate-allowance $RATE_ALLOWANCE \
  --lockup-allowance $LOCKUP_ALLOWANCE \
  --max-lockup-period $MAX_LOCKUP_PERIOD
```

---

### Deposit Tokens

Deposit ERC20 tokens into your account in the Payments contract using EIP-2612 permit.

```bash
service-operator payments deposit --amount <amount> [flags]
```

**Required Flags:**
- `--amount <amount>` - Amount to deposit in base token units

**Optional Flags:**
- `--to <address>` - Address to credit the deposit (default: your address)
- `--permit-deadline <timestamp>` - Unix timestamp for permit expiration (default: 1 hour from now)

**Requirements:**
- You must have the tokens in your wallet
- Token contract must support EIP-2612 permit functionality
- The EIP-2612 permit signature is generated internally using your private key

#### Examples

Deposit 10 USDFC (10,000,000 base units with 6 decimals) to your account:

```bash
export SERVICE_OPERATOR_PRIVATE_KEY="./wallet-key.hex"
export SERVICE_OPERATOR_TOKEN_ADDRESS="0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0"

service-operator payments deposit \
  --network calibration \
  --amount 10000000
```

Deposit to a specific address:

```bash
service-operator payments deposit \
  --amount 10000000 \
  --to 0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb
```

**Note:** Depositing does NOT automatically approve any operators. After depositing, you must separately approve the FilecoinWarmStorageService contract using `approve-service`.

---

### Approve Service as Operator

Approve the FilecoinWarmStorageService contract to operate on your behalf in the Payments contract. This allows the service contract to manage payment rails for your datasets.

```bash
service-operator payments approve-service [flags]
```

**Required Flags:**
- `--rate-allowance <amount>` - Maximum rate (tokens per second) the operator can commit
- `--lockup-allowance <amount>` - Maximum amount the operator can lock up across all rails
- `--max-lockup-period <seconds>` - Maximum lockup period the operator can use

**Optional Flags:**
- `--deposit` - Include a token deposit with the approval (requires permit signature)
- `--amount <amount>` - Amount to deposit (required if --deposit=true)
- `--permit-deadline <timestamp>` - Unix timestamp for permit expiration (default: 1 hour from now)

**Requirements:**
- You must have the required tokens in your wallet (if depositing)
- Token contract must support EIP-2612 permit functionality
- You must have enough FIL for gas fees
- The EIP-2612 permit signature is generated internally using your private key

#### Operator Approval Only (No Deposit)

Approve the service contract without depositing tokens:

```bash
export SERVICE_OPERATOR_PRIVATE_KEY="./wallet-key.hex"
export SERVICE_OPERATOR_TOKEN_ADDRESS="0x..." # Your token contract address

service-operator payments approve-service \
  --network calibration \
  --rate-allowance 1000000 \
  --lockup-allowance 5000000 \
  --max-lockup-period 2592000
```

#### Deposit and Approve in One Transaction

Deposit tokens and approve the operator in a single gasless transaction using EIP-2612 permit:

```bash
export SERVICE_OPERATOR_PRIVATE_KEY="./wallet-key.hex"
export SERVICE_OPERATOR_TOKEN_ADDRESS="0x..." # Your token contract address

service-operator payments approve-service \
  --network calibration \
  --deposit \
  --amount 10000000 \
  --rate-allowance 1000000 \
  --lockup-allowance 5000000 \
  --max-lockup-period 2592000
```

Using explicit contract addresses:

```bash
service-operator payments approve-service \
  --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
  --contract-address 0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91 \
  --payments-address 0x0E690D3e60B0576D01352AB03b258115eb84A047 \
  --token-address 0x... \
  --private-key ./wallet-key.hex \
  --deposit \
  --amount 10000000 \
  --rate-allowance 1000000 \
  --lockup-allowance 5000000 \
  --max-lockup-period 2592000
```

**Understanding Allowances:**

- **Rate Allowance**: The maximum rate (tokens per second) the operator can commit across all payment rails. For example, 1000000 tokens per second means the operator can create rails with combined rates up to this limit.

- **Lockup Allowance**: The maximum total amount the operator can lock up across all payment rails. This limits the total exposure to the operator.

- **Max Lockup Period**: The maximum duration (in seconds) the operator can lock up funds. For example, 2592000 seconds = 30 days.

## Private Key Formats

The tool supports multiple private key formats:

1. **Hex-encoded file** - A file containing the private key in hexadecimal format (with or without `0x` prefix)
2. **Raw bytes file** - A file containing the raw private key bytes
3. **Encrypted keystore** - An encrypted keystore file (requires password)

## Help

Get help on any command:

```bash
service-operator --help
service-operator provider --help
service-operator provider list --help
service-operator provider approve --help
service-operator payments --help
service-operator payments calculate --help
service-operator payments deposit --help
service-operator payments approve-service --help
```

## Future Commands

The CLI is designed to be extensible. Future commands will include:

- `service-operator provider remove <provider-id> <index>` - Remove approved provider
- `service-operator config set-commission <bps>` - Update service commission
- `service-operator config set-proving-period <period> <window>` - Configure proving period
