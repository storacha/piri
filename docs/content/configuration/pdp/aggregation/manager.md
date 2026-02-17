# Manager

Aggregation manager configuration.

| Key                                             | Default            | Env                                                  | Dynamic |
|-------------------------------------------------|--------------------|------------------------------------------------------|---------|
| `pdp.aggregation.manager.poll_interval`         | `30s`              | `PIRI_PDP_AGGREGATION_MANAGER_POLL_INTERVAL`         | Yes     |
| `pdp.aggregation.manager.batch_size`            | `10`               | `PIRI_PDP_AGGREGATION_MANAGER_BATCH_SIZE`            | Yes     |
| `pdp.aggregation.manager.job_queue.workers`     | `runtime.NumCPU()` | `PIRI_PDP_AGGREGATION_MANAGER_JOB_QUEUE_WORKERS`     | No      |
| `pdp.aggregation.manager.job_queue.retries`     | `50`               | `PIRI_PDP_AGGREGATION_MANAGER_JOB_QUEUE_RETRIES`     | No      |
| `pdp.aggregation.manager.job_queue.retry_delay` | `10s`              | `PIRI_PDP_AGGREGATION_MANAGER_JOB_QUEUE_RETRY_DELAY` | No      |

## Overview

The manager is the final stage of the aggregation pipeline before chain submission. 
It buffers completed aggregates and submits them to the blockchain in batches.

**How it works:**

- Aggregates are buffered as they complete
- When `poll_interval` elapses OR `batch_size` is reached, the buffer is submitted
- Submissions are enqueued to a job queue for reliable delivery

**Dynamic Configuration:** Both `poll_interval` and `batch_size` can be changed at runtime using the admin API. 
Changes take effect immediately.

## Fields

### `poll_interval`
The frequency in which the aggregation manager submits roots to the chain.

### `batch_size`
The maximum number of roots to submit in a single message.

### `job_queue.workers`
The number of workers spawned by the manager controlling the number of roots that may be submitted in parallel.

### `job_queue.retries`
The number of times to retry submitting a root before the operation is considered failed.

### `job_queue.retry_delay`

The duration to wait between successive failures.

## Gas Usage

The manager configuration directly affects gas costs for chain submission.

### How batching reduces gas

Each blockchain transaction has:

- **Fixed costs**: Transaction overhead, signature verification
- **Variable costs**: Per-root data storage

By batching multiple roots into a single transaction, you amortize fixed costs across more roots.

### Configuration trade-offs

| Setting | Effect | Gas Impact |
|---------|--------|------------|
| Higher `batch_size` | More roots per transaction | Lower gas per root |
| Lower `batch_size` | Fewer roots per transaction | Higher gas per root |
| Higher `poll_interval` | More time to fill batches | Better batching, lower gas |
| Lower `poll_interval` | Less time to fill batches | Faster finality, potentially higher gas |

### Recommendations

**High-volume nodes** (many blobs/hour):

- Use default or higher `batch_size` (10+)
- Use moderate `poll_interval` (30s-60s)
- Batches will naturally fill, optimizing gas

**Low-volume nodes** (few blobs/hour):

- Consider higher `poll_interval` (60s-300s) to allow batches to fill
- Or accept higher per-root gas cost for faster finality

**Cost-sensitive deployments**:

- Maximize `batch_size` and `poll_interval`
- Trade finality speed for gas efficiency

## TOML

```toml
[pdp.aggregation.manager]
poll_interval = "30s"
batch_size = 10

[pdp.aggregation.manager.job_queue]
workers = 3
retries = 50
retry_delay = "1m"
```
