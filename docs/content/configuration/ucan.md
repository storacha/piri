# ucan

UCAN service configuration.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `ucan.proof_set` | - | `PIRI_UCAN_PROOF_SET` | No |

## Fields

### `proof_set`

Proof set ID from initialization.

## TOML

```toml
[ucan]
proof_set = 123
```

<details>
<summary>Preset-Managed Fields</summary>

These fields are automatically configured by the `network` preset. You typically don't need to set them manually. See [presets](../presets.md) for details.

## [ucan.services]

External service connections.

### [ucan.services.indexer]

```toml
[ucan.services.indexer]
did = "did:web:indexer.forge.storacha.network"
url = "https://indexer.forge.storacha.network/claims"
proof = "..."  # Optional delegation proof
```

### [ucan.services.upload]

```toml
[ucan.services.upload]
did = "did:web:up.forge.storacha.network"
url = "https://up.forge.storacha.network"
```

### [ucan.services.etracker]

Egress tracker for usage reporting.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `ucan.services.etracker.max_batch_size_bytes` | `104857600` | `PIRI_UCAN_SERVICES_ETRACKER_MAX_BATCH_SIZE_BYTES` | No |

Constraints: 10MiB - 1GiB

```toml
[ucan.services.etracker]
did = "did:web:etracker.forge.storacha.network"
url = "https://etracker.forge.storacha.network"
receipts_endpoint = "https://..."
max_batch_size_bytes = 104857600  # 100 MiB
proof = "..."  # Optional
```

### [ucan.services.publisher]

IPNI announcement configuration.

```toml
[ucan.services.publisher]
ipni_announce_urls = [
  "https://cid.contact/announce",
  "https://ipni.forge.storacha.network"
]
```

### [ucan.services.principal_mapping]

Maps service DIDs to principal DIDs.

```toml
[ucan.services]
principal_mapping = { "did:web:up.forge.storacha.network" = "did:key:z6Mk..." }
```

</details>
