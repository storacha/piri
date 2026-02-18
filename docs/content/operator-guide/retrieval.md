# How Retrieval Works

Your Piri node serves data to clients who request it. This page explains how retrieval requests arrive, how your node responds, and how retrievals are tracked for payment.

## The Retrieval Flow

When a client wants to retrieve data:

1. **Discovery**: The client asks Storacha where to find the content
2. **Routing**: Storacha directs them to a node that has it (your node)
3. **Authorization**: The client provides UCAN authorization to your node
4. **Serving**: Your node serves the requested data
5. **Tracking**: Your node records a receipt for this egress event

```
┌────────┐     1. Where is CID X?      ┌──────────┐
│ Client │ ───────────────────────────▶│ Storacha │
└────────┘                             └──────────┘
    │                                       │
    │         2. Node Y has it              │
    │◀──────────────────────────────────────┘
    │
    │         3. UCAN auth + request
    ▼
┌────────┐
│  Piri  │ ◀─── Your node
│  Node  │
└────────┘
    │
    │         4. Data served
    ▼
┌────────┐
│ Client │
└────────┘
```

### Node Selection

The diagram above simplifies reality in the interest of clarity. Due to replication, the astute reader will have noticed that *three* nodes typically hold any given piece of content, not merely "Node Y." Storacha, being honest about the state of the network, returns all nodes known to possess the requested data.

The client then faces a choice—a modest embarrassment of riches. By default, client software selects a node at random, which distributes load across the network with admirable fairness if not particular cleverness.

More sophisticated clients may implement their own selection heuristics:

- **Latency**: Prefer nodes that respond quickly
- **Reliability**: Favor nodes with consistent uptime and successful retrieval history
- **Geographic proximity**: Select nodes that are physically closer to reduce round-trip time

This arrangement creates a quiet but persistent incentive for operators. Nodes that perform well—responding promptly and reliably—may find themselves preferentially selected by discerning clients. Nodes that do not may discover, over time, that their services are requested less frequently. The network, in its decentralized wisdom, rewards competence.

Retrieval serving is automatic—your node handles requests without intervention.

## Serving Data

Your node serves data over HTTP. Clients typically request specific byte ranges rather than entire blobs.

### Range Requests

Blobs stored in Piri are typically 256MiB (expanding to 512MiB). Clients often need only portions of this data, so they use HTTP range headers to request specific byte ranges.

Your node handles range requests automatically. This is efficient for both parties:

- Clients download only what they need
- Your node serves only what's requested
- Egress is calculated on actual bytes served

### Content Addressing

Clients request data by its content identifier (CID)—a cryptographic hash of the content. This ensures they receive exactly what they asked for, verified by the hash.

## Egress Tracking

Every retrieval is tracked for payment purposes.

### What Gets Tracked

When your node serves data, it retains the UCAN token the client presented. This token is cryptographic proof that the client requested the data—it cannot be forged or disputed.

Your node submits these tokens to Storacha's egress tracker service. This happens automatically in the background.

### Mutual Accountability

Neither party grades their own homework:

- **Your node** reports what it served (bytes delivered)
- **The client** reports what it downloaded
- **Storacha** reconciles both reports

This mutual reporting ensures accurate egress accounting. Discrepancies are resolved through reconciliation.

## What Counts as Billable Egress

Not all data leaving your node is billable:

| Traffic Type | Billable |
|--------------|----------|
| Client retrievals | Yes |
| Replication to other nodes | No |

**Client retrievals** are compensated at the egress rate. See [Getting Paid](payments.md) for current rates.

**Replication transfers** between Piri nodes do not count as billable egress. However, receiving replicated data means your node now holds content that clients may request directly—and client retrievals do generate egress revenue.

## Authorization

Clients must present valid UCAN authorization to retrieve data. This ensures:

- Only authorized parties can access content
- Access can be scoped and time-limited
- Your node has cryptographic proof of authorization

Your node validates the UCAN before serving data. Invalid or expired authorizations are rejected.

## Performance Considerations

Retrieval performance affects your node's reputation and may influence future selection for uploads.

**What you control:**

- Disk I/O performance (SSD vs HDD)
- Network bandwidth and connectivity
- Server resources (CPU, memory)

**What you don't control:**

- Client connection quality
- Geographic distance to clients
- Internet routing

Optimizing what you control improves retrieval latency and throughput.
