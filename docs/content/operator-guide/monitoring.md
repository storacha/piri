# Monitoring Your Node

Day-to-day operation involves watching for issues before they become problems. This page covers what to monitor and how.

## Key Health Indicators

### Lotus Sync Status

Your Lotus node must stay synced. If it falls behind, your Piri node can't track epochs correctly and may miss proof windows.

Check Lotus sync:

```bash
lotus sync status
```

Look for the chain height and sync status. If significantly behind the network height, investigate immediately.

### Proof Set State

Your proof set state tells you whether proving is healthy:

```bash
piri client pdp proofset state
```

Watch for:

- **Challenge issued but not proven**: Your node should generate proofs promptly
- **Fault state**: Indicates a missed challenge window—investigate why
- **Epochs until next challenge**: Shows how much time before the next proof is due

### Job Queue Health

Monitor your job queues for stuck or failed jobs:

| Queue | Purpose |
|-------|---------|
| `replicator` | Data replication transfers |
| `aggregator` | Piece aggregation |
| `egress_tracker` | Retrieval event submission |

A growing backlog or high failure rate indicates problems. Check logs for error details.

### Disk Space

Monitor free space in your data directory:

```bash
df -h /path/to/data_dir
```

Running out of space will cause failures. Set up alerts well before capacity is reached.

### System Resources

Basic system health:

- **CPU**: Sustained high usage may indicate resource constraints
- **Memory**: Watch for memory pressure or swapping
- **Network**: Ensure bandwidth is sufficient for replication and retrieval

## Telemetry

Piri emits OpenTelemetry metrics and traces for detailed observability.

### Key Metrics

| Metric | What It Tells You |
|--------|-------------------|
| `active_jobs` | Currently running jobs per queue |
| `queued_jobs` | Jobs waiting in queue |
| `failed_jobs` | Permanently failed jobs (investigate these) |
| `job_duration` | How long jobs take |
| `system_cpu_utilization` | CPU usage |
| `system_memory_used_bytes` | Memory usage |
| `piri_datadir_free_bytes` | Available disk space |
| `chain_current_epoch` | Current Filecoin epoch |
| `next_challenge_window_start_epoch` | When next challenge starts |

### Setting Up Metrics Collection

Configure a metrics endpoint:

```toml
[[telemetry.metrics]]
endpoint = "http://your-collector:4317"
insecure = true
publish_interval = "30s"
```

Send metrics to Prometheus, Grafana, or any OTLP-compatible backend.

See [Concepts > Telemetry](../concepts/telemetry.md) for the complete metrics reference.

## Logs

Piri logs operational events. Adjust log levels dynamically:

```bash
# Increase verbosity for a subsystem
piri client admin log set pdp debug

# List all log subsystems
piri client admin log list
```

When troubleshooting, increase verbosity for the relevant subsystem, reproduce the issue, then review logs.

## Health Endpoint

Your node exposes a health endpoint:

```bash
curl https://your-node.example.com/health
```

Use this for load balancer health checks or uptime monitoring.

## Alerts to Configure

Recommended alerts:

| Condition | Severity | Action |
|-----------|----------|--------|
| Lotus sync behind by >100 epochs | Critical | Check Lotus node immediately |
| Proof set in fault state | Critical | Investigate missed proof |
| Disk space <10% free | Warning | Expand storage or clean up |
| Disk space <5% free | Critical | Immediate action required |
| Failed jobs accumulating | Warning | Check logs for root cause |
| No proofs submitted in proving period | Critical | Verify node is running and healthy |

## Regular Checks

**Daily:**

- Verify Lotus is synced
- Check proof set state for faults
- Review failed job counts

**Weekly:**

- Review disk space trends
- Check for software updates (`piri status upgrade-check`)
- Verify wallet balance for gas

**Monthly:**

- Review overall job success rates
- Check egress payment status
- Consider settling storage payments if accumulated

## Troubleshooting

### Proofs Not Submitting

1. Check Lotus sync status
2. Verify wallet has FIL for gas
3. Check proof set state for errors
4. Review PDP task logs

### Replication Failing

1. Check replicator queue for stuck jobs
2. Verify network connectivity to source
3. Check disk space
4. Review replicator logs

### High Job Failure Rate

1. Identify which queue is failing
2. Check logs for specific error messages
3. Verify external dependencies (Lotus, network, disk)
4. Check if issues are transient or persistent

For detailed troubleshooting, see the specific subsystem's documentation and logs.
