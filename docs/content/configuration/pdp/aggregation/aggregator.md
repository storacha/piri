# Aggregator

Piece aggregation configuration.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `pdp.aggregation.aggregator.job_queue.workers` | `runtime.NumCPU()` | `PIRI_PDP_AGGREGATION_AGGREGATOR_JOB_QUEUE_WORKERS` | No |
| `pdp.aggregation.aggregator.job_queue.retries` | `50` | `PIRI_PDP_AGGREGATION_AGGREGATOR_JOB_QUEUE_RETRIES` | No |
| `pdp.aggregation.aggregator.job_queue.retry_delay` | `10s` | `PIRI_PDP_AGGREGATION_AGGREGATOR_JOB_QUEUE_RETRY_DELAY` | No |

## Overview

The aggregator groups pieces into aggregates between 128MB and 256MB. 
When a piece's CommP hash is calculated, it enters this queue to be combined with other pieces into an aggregate.

**How aggregation works:**

- Pieces are buffered until total size reaches 128MB minimum
- Pieces larger than 128MB are submitted immediately as single-piece aggregates
- The maximum aggregate size is 256MB

**Performance Note:** Aggregate creation involves building merkle trees from piece commitments (32-byte hashes). 
Memory usage is minimal since only the CommP hashes are held in memory, not the actual blob data.

## Fields

### `job_queue.workers`

Number of concurrent aggregation operations. Defaults to the number of CPU cores.

- **Higher values**: Faster aggregate creation when many pieces complete CommP calculation simultaneously
- **Lower values**: Reduced concurrency, but memory impact is minimal since only piece hashes are processed

### `job_queue.retries`

Maximum retry attempts before a piece is moved to the dead-letter queue.

### `job_queue.retry_delay`

Wait time between retry attempts after a failure.

## TOML

```toml
[pdp.aggregation.aggregator.job_queue]
workers = 2
retries = 50
retry_delay = "10s"
```
