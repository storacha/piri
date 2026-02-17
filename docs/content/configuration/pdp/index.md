# pdp

PDP (Provable Data Possession) service configuration.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `pdp.owner_address` | - | `PIRI_PDP_OWNER_ADDRESS` | No |
| `pdp.lotus_endpoint` | - | `PIRI_PDP_LOTUS_ENDPOINT` | No |

## Fields

### `owner_address`

Ethereum address that owns the proof set.

### `lotus_endpoint`

Filecoin Lotus node WebSocket endpoint.

## TOML

```toml
[pdp]
owner_address = "0x1234..."
lotus_endpoint = "wss://lotus.example.com/rpc/v1"
```

### [aggregation](aggregation/index.md)

Aggregation system configuration.

<details markdown="1">
<summary>Preset-Managed Fields</summary>

These fields are automatically configured by the `network` preset. You almost never should set them manually. See [Networks](../../concepts/networks.md) for details.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `pdp.chain_id` | - | `PIRI_PDP_CHAIN_ID` | No |
| `pdp.payer_address` | - | `PIRI_PDP_PAYER_ADDRESS` | No |

### `chain_id`

Filecoin chain ID. `314` for mainnet, `314159` for calibration.

### `payer_address`

Ethereum address that pays storage providers.

## pdp.signing_service

Configure transaction signing. Use either remote signing service OR local private key (not both).

**Remote signing (recommended for production):**

```toml
[pdp.signing_service]
did = "did:web:signer.forge.storacha.network"
url = "https://signer.forge.storacha.network"
```

**Local signing (development only):**

```toml
[pdp.signing_service]
private_key = "0x..."  # Hex-encoded ECDSA private key
```

## pdp.contracts

Smart contract addresses.

```toml
[pdp.contracts]
verifier = "0x..."
provider_registry = "0x..."
service = "0x..."
service_view = "0x..."
```

</details>
