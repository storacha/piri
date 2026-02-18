# How Storage Works

Your Piri node receives data from clients and participates in Storacha's distributed storage network. This page explains how data arrives, how replication maintains redundancy, and what your node actually stores.

## Direct Upload Flow

When a client uploads data to Storacha, your node may be selected to receive it. The data flows directly from the client to your node—Storacha orchestrates the process but never handles the actual bytes.

The upload sequence:

1. **Allocation request**: Client requests an allocation from Storacha, providing the SHA hash and size of the data
2. **Node selection**: Storacha selects a Piri node (see [Node Selection](#node-selection) below)
3. **Allocation creation**: Storacha requests an allocation from the selected node
4. **URL generation**: Your node creates an allocation record and returns an upload URL with authentication tokens
5. **URL delivery**: Storacha sends the upload URL to the client
6. **Direct transfer**: Client performs an HTTP PUT directly to your node
7. **Confirmation**: Client confirms upload completion to Storacha
8. **Notification**: Storacha notifies your node that the upload is complete
9. **Verification**: Your node verifies the data matches the expected CID, computes CommPv2, and queues the root for chain registration

The critical point: actual data transfer is direct from client to your node. Storacha coordinates; your node receives.

## Replication

The Forge network maintains redundancy—three copies of each piece across different nodes. Storacha orchestrates this replication at its layer; your node participates without needing to know whether it's storing a primary copy or a replica.

### How Replication Works

After a client completes a primary upload, Storacha selects additional nodes for redundancy. Each replica follows the same allocation flow: Storacha requests an allocation, your node returns a URL, and data transfers in.

From your node's perspective, there is no difference between a primary upload and a replica. Both proceed through the identical pipeline:

1. Receive allocation request
2. Generate upload URL
3. Accept data transfer
4. Verify and compute CommPv2
5. Queue for chain submission

### What This Means for Operators

- Your node may receive data originally uploaded to another node
- Compensation is identical regardless of primary versus replica status—see [Getting Paid](payments.md) for rates
- Replication traffic between nodes is not billable egress
- Storage and proof obligations are the same for all copies

Storacha selects which nodes participate in replication using the same selection mechanism as primary uploads. Data transfers directly between nodes.

## Node Selection

Storacha selects which nodes receive uploads and replicas. The current algorithm uses weighted random selection with uniform weights—all active, healthy nodes have equal probability of selection.

### Future Considerations

Selection may evolve to consider:

- **Throughput history**: Faster nodes may receive more allocations
- **Reliability**: Fewer failures could increase preference
- **Geographic optimization**: Proximity to clients may factor in
- **Capacity**: Nodes with more available space may receive larger allocations

Operators who invest in performance and reliability may benefit as the selection algorithm matures.

## What Gets Stored

Your node stores several types of data:

**Blobs** are chunks of content identified by their cryptographic hash (content addressing). User data constitutes the bulk of storage usage.

**Location claims** are also blobs, but with different content—they record where data can be retrieved. When your node accepts user data, it creates a claim asserting "this content is available at this URL."

In practice, almost all blobs uploaded to Piri are 256MiB. This will eventually expand to 512MiB—no operator action required, but it helps to understand the rate and size of data moving through your system.

## Aggregation

Before data can be proven on-chain, it must be aggregated into larger units with cryptographic commitments:

1. **Pieces arrive**: Individual blobs are accepted and queued for aggregation
2. **CommP calculation**: Pieces are combined and a commitment hash (CommP) is calculated. This is CPU-intensive—modern processors with SHA extensions significantly improve performance.
3. **Root creation**: The aggregated commitment becomes a "root" in your data set
4. **On-chain registration**: The root is added to your data set on-chain

The aggregation manager runs in the background, periodically checking for pending pieces and batching them together. The polling interval and batch size are tunable—see [aggregation manager configuration](../configuration/pdp/aggregation/manager.md) and [dynamic configuration](../configuration/index.md#dynamic-configuration) for details.

## What Gets Stored Where

Piri manages several categories of data:

- **Blobs**: User data and location claims
- **Aggregation state**: CommP calculations and pending aggregates
- **Publisher metadata**: IPNI publication records
- **Receipts**: UCAN receipt journals for egress tracking

The storage backend handles persistence. Configure storage paths during initialization or via the `repo.data_dir` configuration option.

## Timing Considerations

Some performance factors are within your control; others are not.

**What operators control:**

- Kernel tuning and network stack configuration
- Network peering and connectivity
- Hardware selection (SHA extensions accelerate hashing)
- Disk I/O performance

**What operators do not control:**

- Client bandwidth and connection quality
- Client geographic location
- Internet routing between client and node
- Storacha's selection decisions

Optimizing what you control improves your node's performance profile, which may influence selection as the algorithm evolves.

## Capacity Planning

Consider these factors when sizing storage:

- **Data growth**: Storage usage grows as you accept more data
- **Retention**: Data remains until explicitly removed (rare in normal operation)
- **Headroom**: Maintain buffer space for aggregation working files and database growth
- **Backup**: Consider your backup strategy for stored data

### Scaling Options

As capacity needs grow, you have options:

- **Multiple nodes**: Run additional Piri instances, each managing its own storage
- **Scalable storage backend**: Use a storage backend that can grow with your needs

### Database Selection

Piri uses a database for operational state and job scheduling. Choose based on your scale and requirements—see [Database](../concepts/database.md) for details on SQLite versus PostgreSQL.

The [Prerequisites](../setup/prerequisites.md) guide recommends 1+ TB to start, but actual needs depend on how much data Storacha routes to your node.
