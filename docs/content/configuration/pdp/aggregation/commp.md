# CommP

CommP (Piece Commitment) configuration.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `pdp.aggregation.commp.job_queue.workers` | `runtime.NumCPU()` | `PIRI_PDP_AGGREGATION_COMMP_JOB_QUEUE_WORKERS` | No |
| `pdp.aggregation.commp.job_queue.retries` | `50` | `PIRI_PDP_AGGREGATION_COMMP_JOB_QUEUE_RETRIES` | No |
| `pdp.aggregation.commp.job_queue.retry_delay` | `10s` | `PIRI_PDP_AGGREGATION_COMMP_JOB_QUEUE_RETRY_DELAY` | No |

## Overview

CommP calculation is the first stage of the aggregation pipeline. When a blob is uploaded, it enters this queue to have 
its Piece Commitment hash calculated. This hash is required for creating aggregates that can be submitted to the 
blockchain.

**Performance Note:** CommP calculation is CPU-intensive. Each worker consumes significant CPU resources while processing a blob.

## Fields

### `job_queue.workers`

Number of concurrent CommP calculations. Defaults to the number of CPU cores.

- **Higher values**: Faster throughput when many blobs arrive simultaneously, but higher CPU usage
- **Lower values**: Reduced CPU load, but blobs queue longer during high ingest periods

### `job_queue.retries`

Maximum retry attempts before a blob is moved to the dead-letter queue.

### `job_queue.retry_delay`

Wait time between retry attempts after a failure.

## TOML

```toml
[pdp.aggregation.commp.job_queue]
workers = 4        # Limit to 4 cores for shared environments
retries = 50
retry_delay = "10s"
```
