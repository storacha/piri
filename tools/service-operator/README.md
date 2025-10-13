# Service Operator CLI

Command-line tool for managing FilecoinWarmStorageService smart contracts, handling provider approvals and payment operations.

## Installation

```bash
cd tools/service-operator
go build -o service-operator
```

## Configuration

Configure via command-line flags, environment variables, or YAML config file (`./service-operator.yaml`).

### Environment Variables

Prefix all variables with `SERVICE_OPERATOR_`:

```bash
SERVICE_OPERATOR_RPC_URL                # Ethereum RPC endpoint (required)
SERVICE_OPERATOR_CONTRACT_ADDRESS       # FilecoinWarmStorageService contract address
SERVICE_OPERATOR_PAYMENTS_ADDRESS       # Payments contract address
SERVICE_OPERATOR_TOKEN_ADDRESS          # ERC20 token contract (must support EIP-2612)
SERVICE_OPERATOR_PRIVATE_KEY            # Path to private key file
SERVICE_OPERATOR_KEYSTORE               # Path to keystore file (alternative to private key)
SERVICE_OPERATOR_KEYSTORE_PASSWORD      # Keystore password
```

### Private Key Formats

Supports:
- Hex-encoded file (with or without `0x` prefix)
- Raw bytes file
- Encrypted keystore (requires password)

### Example Config File

```yaml
# service-operator.yaml
rpc_url: "https://api.calibration.node.glif.io/rpc/v1"
contract_address: "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91"
payments_address: "0x6dB198201F900c17e86D267d7Df82567FB03df5E"
token_address: "0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0"
keystore: "./my-keystore"
keystore_password: "password"
```

## Provider Commands

### List Providers

List registered service providers from ServiceProviderRegistry.

```bash
service-operator provider list [flags]
```

**Flags:**
- `--limit <n>` - Max providers to display (default: 50)
- `--offset <n>` - Pagination offset (default: 0)
- `--show-inactive` - Include inactive providers
- `--format <table|json>` - Output format

**Note:** Read-only operation; only RPC URL required.

**Example:**
```bash
service-operator provider list \
  --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
  --format json --limit 100
```

### Approve Provider

Approve a provider to create datasets in FilecoinWarmStorageService.

```bash
service-operator provider approve <provider-id>
```

**Requirements:** Contract owner credentials, sufficient FIL for gas.

**Example:**
```bash
export SERVICE_OPERATOR_PRIVATE_KEY="./owner-key.hex"
export SERVICE_OPERATOR_RPC_URL="https://api.calibration.node.glif.io/rpc/v1"
export SERVICE_OPERATOR_CONTRACT_ADDRESS="0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91"

service-operator provider approve 123
```

## Payment Commands

### Calculate Allowances

Calculate rate allowance, lockup allowance, and max lockup period for operator approval.

```bash
service-operator payments calculate --size <size> [flags]
```

**Flags:**
- `--size <size>` - Dataset size (e.g., 1TiB, 500GiB, 2.5TiB) **[required]**
- `--lockup-days <days>` - Lockup period in days (default: 10)
- `--max-lockup-period-days <days>` - Max lockup period in days (default: 30)
- `--format <human|shell|flags>` - Output format (default: human)

**Example:**
```bash
# Calculate for 1 TiB, export as env vars
eval $(service-operator payments calculate --size 1TiB --format shell)

# Use calculated values
service-operator payments approve-service \
  --rate-allowance $RATE_ALLOWANCE \
  --lockup-allowance $LOCKUP_ALLOWANCE \
  --max-lockup-period $MAX_LOCKUP_PERIOD
```

### Balance

Display USDFC token balance in your wallet (not in Payments contract).

```bash
service-operator payments balance
```

Shows funds available to deposit into the Payments contract.

**Example:**
```bash
service-operator payments balance \
  --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
  --token-address 0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0 \
  --private-key ./wallet-key.hex
```

### Account

Display account balance in the Payments contract.

```bash
service-operator payments account [flags]
```

**Flags:**
- `--address <addr>` - Address to check (defaults to keystore address)

Shows total funds, locked funds, and available funds that can be withdrawn. Useful for storage providers checking settlement earnings.

**Example:**
```bash
# Check your own balance
service-operator payments account --keystore ./my-keystore

# Check any address (read-only)
service-operator payments account \
  --address 0x7469B47e006D0660aB92AE560b27A1075EEcF97F \
  --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
  --payments-address 0x6dB198201F900c17e86D267d7Df82567FB03df5E \
  --token-address 0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0
```

### Status

Comprehensive view of account balance, operator approval status, and active payment rails.

```bash
service-operator payments status
```

Displays:
- Account balance (funds/lockup)
- Operator approval (allowances/usage)
- Available capacity for new rails
- Active payment rails with IDs

**Example:**
```bash
service-operator payments status \
  --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
  --payments-address 0x6dB198201F900c17e86D267d7Df82567FB03df5E \
  --token-address 0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0 \
  --contract-address 0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91 \
  --private-key ./wallet-key.hex
```

### Deposit

Deposit ERC20 tokens into Payments contract using EIP-2612 permit.

```bash
service-operator payments deposit --amount <amount> [flags]
```

**Flags:**
- `--amount <amount>` - Amount in base token units **[required]**
- `--to <address>` - Address to credit (default: your address)
- `--permit-deadline <timestamp>` - Permit expiration (default: 1 hour from now)

**Requirements:** Tokens in wallet, EIP-2612 support, sufficient FIL for gas.

**Note:** Does NOT automatically approve operators. Use `approve-service` separately.

**Example:**
```bash
# Deposit 10 USDFC (10,000,000 base units with 6 decimals)
export SERVICE_OPERATOR_TOKEN_ADDRESS="0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0"
export SERVICE_OPERATOR_PRIVATE_KEY="./wallet-key.hex"

service-operator payments deposit \
  --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
  --payments-address 0x6dB198201F900c17e86D267d7Df82567FB03df5E \
  --amount 10000000
```

### Approve Service

Approve FilecoinWarmStorageService contract as operator in Payments contract.

```bash
service-operator payments approve-service [flags]
```

**Required Flags:**
- `--rate-allowance <amount>` - Max rate (tokens/second) operator can commit
- `--lockup-allowance <amount>` - Max total lockup across all rails
- `--max-lockup-period <seconds>` - Max lockup period duration

**Optional Flags:**
- `--deposit` - Include token deposit with approval
- `--amount <amount>` - Deposit amount (required if `--deposit`)
- `--permit-deadline <timestamp>` - Permit expiration

**Understanding Allowances:**
- **Rate Allowance**: Max tokens/second across all rails
- **Lockup Allowance**: Max total locked amount across all rails
- **Max Lockup Period**: Max duration (seconds) for locking funds

**Examples:**

Approve only (no deposit):
```bash
service-operator payments approve-service \
  --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
  --payments-address 0x6dB198201F900c17e86D267d7Df82567FB03df5E \
  --contract-address 0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91 \
  --token-address 0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0 \
  --private-key ./wallet-key.hex \
  --rate-allowance 57 \
  --lockup-allowance 1641600 \
  --max-lockup-period 86400
```

Deposit and approve in one transaction:
```bash
service-operator payments approve-service \
  --rpc-url https://api.calibration.node.glif.io/rpc/v1 \
  --payments-address 0x6dB198201F900c17e86D267d7Df82567FB03df5E \
  --contract-address 0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91 \
  --token-address 0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0 \
  --private-key ./wallet-key.hex \
  --deposit \
  --amount 10000000 \
  --rate-allowance 57 \
  --lockup-allowance 1641600 \
  --max-lockup-period 86400
```

### Settle

Settle payment rails to transfer locked funds from payer to payee.

```bash
service-operator payments settle [flags]
```

**Flags:**
- `--rail-id <id>` - Specific rail to settle
- `--all` - Settle all rails for service provider
- `--until-epoch <epoch>` - Settle up to epoch (default: current block)

Settlement triggers FilecoinWarmStorageService to validate PDP proofs and pay only for proven epochs.

**Network Fee:** Requires 0.0013 FIL as network fee.

**Examples:**
```bash
# Settle specific rail
service-operator payments settle --rail-id 1

# Settle specific rail up to epoch
service-operator payments settle --rail-id 1 --until-epoch 1000000

# Settle all rails
service-operator payments settle --all
```

### Withdraw

Withdraw available funds from Payments contract to wallet or another address.

```bash
service-operator payments withdraw --amount <amount> [flags]
```

**Flags:**
- `--amount <amount>` - Amount in base units **[required]**
- `--to <address>` - Destination address (default: your address)

Only non-locked funds can be withdrawn. Typically used by storage providers to withdraw settlement earnings.

**Examples:**
```bash
# Withdraw to own address using keystore
service-operator payments withdraw \
  --amount 1329414936966 \
  --keystore ./my-keystore \
  --keystore-password password

# Withdraw to specific address
service-operator payments withdraw \
  --amount 1329414936966 \
  --to 0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0 \
  --private-key ./wallet-key.hex
```

## Help

```bash
service-operator --help
service-operator [command] --help
service-operator payments [subcommand] --help
service-operator provider [subcommand] --help
```
