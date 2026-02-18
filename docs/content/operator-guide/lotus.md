# Lotus Node Setup

Piri requires a Filecoin Lotus node to interact with the blockchain. Your Lotus node provides chain data for proof scheduling and submits transactions for proof verification.

## Requirements

### Hardware

Lotus has the following [resource requirements](https://lotus.filecoin.io/lotus/install/prerequisites/#minimal-requirements):

| Resource | Requirement                                                          |
|----------|----------------------------------------------------------------------|
| CPU      | 8+ cores (AMD Zen or Intel Ice Lake+ recommended for SHA extensions) |
| RAM      | 32+ GB                                                               |
| Storage  | preferably on an SSD storage medium; chain grows ~38 GB/day          |
| OS       | Linux or macOS (Windows not supported)                               |

**Deployment options:**

1. **Dedicated machine** (recommended): Run Lotus on its own server. This isolates Lotus resource usage from Piri and simplifies troubleshooting.

2. **Combined machine**: Run Lotus and Piri together if you have sufficient resources. Add Lotus requirements to [Piri's requirements](../setup/prerequisites.md) when sizing your machine.

3. **Managed Lotus service**: Use a third-party Lotus RPC provider if you prefer not to run your own node. The provider must support WebSocket connections and expose the following APIs:

   **Filecoin API:** `ChainHead`, `ChainNotify`, `StateGetRandomnessDigestFromBeacon`

   **Ethereum RPC:** `eth_chainId`, `eth_getBlockByNumber`, `eth_getTransactionCount`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_maxPriorityFeePerGas`, `eth_getTransactionByHash`, `eth_getTransactionReceipt`

### Correct Version

Use a Lotus version compatible with your target network:

- **Mainnet** (`forge-prod`): Use the latest stable Lotus release
- **Calibration** (`warm-staging`): Use a Lotus version that supports the Calibration testnet

Check the [Lotus releases](https://github.com/filecoin-project/lotus/releases) for version compatibility.

### WebSocket RPC

Piri connects to Lotus via WebSocket for real-time chain event subscriptions:

- **Secure (recommended)**: `wss://your-lotus-node.example.com/rpc/v1`
- **Insecure (development only)**: `ws://localhost:1234/rpc/v1`

Use `wss://` (WebSocket over TLS) in production for security.

### Ethereum RPC

Piri interacts with smart contracts on Filecoin and **requires Ethereum RPC to be enabled** in your Lotus node. Without this, Piri cannot function.

You must enable both `EnableEthRPC` and `ChainIndexer` in your Lotus configuration. The ChainIndexer maintains the indices required for Ethereum-compatible queries.

See [Ethereum RPC](https://lotus.filecoin.io/lotus/configure/ethereum-rpc/) for configuration instructions.

## Chain Snapshots

Syncing a Lotus node from genesis is impractical—it takes at least one month to validate the full chain history. 
Instead, sync from a lightweight snapshot.

Lightweight snapshots contain recent block data and state but exclude full historical data, making them suitable for most operators.

See [Chain Management](https://lotus.filecoin.io/lotus/manage/chain-management/) for snapshot download and import instructions.

The [Forest Archive](https://forest-archive.chainsafe.dev/list) maintains historical snapshots for approximately one month. Downloading an older snapshot can help when recovering from extended downtime, as it may contain chain state that newer snapshots lack.

## Splitstore and Chain History

Lotus uses [splitstore](https://lotus.filecoin.io/lotus/configure/splitstore/) to manage chain data efficiently. A key setting is `HotStoreMessageRetention`, which controls how much chain history Lotus keeps available.

**Why this matters for Piri:**

Piri queries Lotus for chain state during operation. Problems occur when Piri needs state that Lotus no longer has:

- **Piri offline too long**: After extended downtime, Piri may query for chain state that Lotus has garbage collected.
- **Lotus recovery**: When Lotus fails and you reimport a snapshot, that snapshot may not contain the state Piri was waiting on.

The `HotStoreMessageRetention` setting (measured in finalities) determines your tolerance for these scenarios. Higher retention means more disk usage but a longer recovery window.

See the [Splitstore documentation](https://lotus.filecoin.io/lotus/configure/splitstore/) for configuration details.

### Redundancy

For higher availability, some operators run multiple Lotus nodes with load balancing. This provides resilience if one node fails.

Operators with an appetite for collaboration might consider a more ambitious approach: pooling resources with other Forge network participants to maintain shared Lotus infrastructure. A collectively-operated cluster could distribute both the operational burden and the costs across multiple parties, potentially achieving reliability levels that would be impractical for any single operator to maintain alone.

## Verifying Your Setup

### Check Sync Status

```bash
lotus sync status
```

Look for output showing the chain is synced to the current height. If behind, wait for sync to complete before starting Piri.

### Check WebSocket Endpoint

```bash
# Test with wscat or similar tool
wscat -c ws://localhost:1234/rpc/v1
```

A successful connection indicates the endpoint is accessible.

### Common Issues

**Node not syncing:**

- Check network connectivity
- Verify you have sufficient disk space
- Ensure you're connecting to the correct network (mainnet vs calibration)

**WebSocket connection refused:**

- Verify Lotus API is enabled and listening
- Check firewall rules allow the connection
- Confirm the endpoint URL is correct

**Authentication errors:**

- Lotus requires an API token for some operations
- Generate a token with `lotus auth create-token --perm admin`
- Include the token in your connection URL or configuration

## Piri Configuration

Once Lotus is running, configure Piri to connect:

```toml
[pdp]
lotus_endpoint = "wss://your-lotus-node.example.com/rpc/v1"
```

Or via environment variable:

```bash
export PIRI_PDP_LOTUS_ENDPOINT="wss://your-lotus-node.example.com/rpc/v1"
```

## Resources

- [Lotus Prerequisites](https://lotus.filecoin.io/lotus/install/prerequisites/) — Hardware requirements and installation
- [Chain Management](https://lotus.filecoin.io/lotus/manage/chain-management/) — Syncing and snapshot management
- [Splitstore Configuration](https://lotus.filecoin.io/lotus/configure/splitstore/) — Managing chain data storage
- [Ethereum RPC](https://lotus.filecoin.io/lotus/configure/ethereum-rpc/) — Required for Piri smart contract interaction
- [Calibration Testnet](https://docs.filecoin.io/networks/calibration) — Testnet information
