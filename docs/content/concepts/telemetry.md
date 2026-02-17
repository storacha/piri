# Telemetry

Piri uses [OpenTelemetry](https://opentelemetry.io/) to emit metrics and traces for observability. You can configure custom collectors to send this data to your own monitoring infrastructure.

## Metrics

Piri emits metrics via OTLP (OpenTelemetry Protocol) that can be consumed by any compatible collector.

### Host Metrics

System-level metrics for monitoring node health:

| Metric                                   | Type  | Unit  | Description                         |
|------------------------------------------|-------|-------|-------------------------------------|
| <nobr>`system_cpu_utilization`</nobr>    | Gauge | 0-1   | System-wide CPU utilization         |
| <nobr>`system_memory_used_bytes`</nobr>  | Gauge | bytes | System memory in use                |
| <nobr>`system_memory_total_bytes`</nobr> | Gauge | bytes | Total system memory                 |
| <nobr>`piri_datadir_used_bytes`</nobr>   | Gauge | bytes | Disk space used by data directory   |
| <nobr>`piri_datadir_free_bytes`</nobr>   | Gauge | bytes | Free disk space for data directory  |
| <nobr>`piri_datadir_total_bytes`</nobr>  | Gauge | bytes | Total disk space for data directory |

### Job Queue Metrics

Track task execution in internal job queues:

| Metric                      | Type          | Description                      |
|-----------------------------|---------------|----------------------------------|
| <nobr>`active_jobs`</nobr>  | UpDownCounter | Currently running jobs           |
| <nobr>`queued_jobs`</nobr>  | UpDownCounter | Jobs waiting in queue            |
| <nobr>`failed_jobs`</nobr>  | Counter       | Permanently failed jobs          |
| <nobr>`job_duration`</nobr> | Histogram     | Job execution duration (seconds) |

**Labels:**

| Label | Description |
|-------|-------------|
| `queue` | Name of the job queue (e.g., `replicator`, `aggregator`, `egress_tracker`) |
| `job` | Type of job being executed |
| `status` | Job outcome (`success` or `failure`) |
| `attempt` | Retry attempt number (1-based) |
| `failure_reason` | Reason for permanent failure (only on `failed_jobs`) |

### HTTP Server Metrics

Standard OpenTelemetry HTTP instrumentation:

| Metric                                        | Type      | Description        |
|-----------------------------------------------|-----------|--------------------|
| <nobr>`http.server.request.duration`</nobr>   | Histogram | Request latency    |
| <nobr>`http.server.request.body.size`</nobr>  | Histogram | Request body size  |
| <nobr>`http.server.response.body.size`</nobr> | Histogram | Response body size |

### PDP Metrics

Provable Data Possession task metrics:

| Metric                                           | Type    | Description                             |
|--------------------------------------------------|---------|-----------------------------------------|
| <nobr>`chain_current_epoch`</nobr>               | Gauge   | Current Filecoin chain epoch            |
| <nobr>`next_challenge_window_start_epoch`</nobr> | Gauge   | Epoch when next challenge window starts |
| <nobr>`pdp_next_failure`</nobr>                  | Counter | Next proving period task failures       |
| <nobr>`pdp_prove_failure`</nobr>                 | Counter | Proof generation task failures          |
| <nobr>`message_send_failure`</nobr>              | Counter | Blockchain message send failures        |
| <nobr>`message_estimate_gas_failure`</nobr>      | Counter | Gas estimation failures                 |

### Replication Metrics

| Metric                           | Type      | Description                         |
|----------------------------------|-----------|-------------------------------------|
| <nobr>`transfer_duration`</nobr> | Histogram | Replica transfer operation duration |

**Labels:**

| Label | Description |
|-------|-------------|
| `source` | Origin endpoint where data is pulled from |
| `sink` | Destination endpoint where data is written to (this node) |

### Server Info

Build and runtime information:

| Metric                          | Type | Description     |
|---------------------------------|------|-----------------|
| <nobr>`piri_server_info`</nobr> | Info | Server metadata |

**Labels:**

| Label | Description |
|-------|-------------|
| `version` | Piri software version |
| `commit` | Git commit hash of the build |
| `built_by` | Build system identifier |
| `build_date` | When the binary was compiled |
| `start_time_unix` | Server start time (Unix timestamp) |
| `server_type` | Server mode (`full` or `ucan`) |
| `did` | Server's Decentralized Identifier |
| `owner_address` | Ethereum address of node owner |
| `public_url` | Server's publicly accessible URL |
| `proof_set` | PDP proof set ID |

## Traces

Distributed tracing provides end-to-end visibility into operations:

| Span                                  | Description                  |
|---------------------------------------|------------------------------|
| <nobr>`blob.accept`</nobr>            | Blob acceptance operations   |
| <nobr>`blob.allocate`</nobr>          | Blob allocation operations   |
| <nobr>`space.content.retrieve`</nobr> | Content retrieval operations |
| <nobr>`AddRoots`</nobr>               | PDP root addition operations |

Traces use parent-based sampling and integrate with W3C Trace Context propagation.

## Integration

### Prometheus

Use an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) with a Prometheus exporter:

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: "0.0.0.0:4317"

exporters:
  prometheus:
    endpoint: "0.0.0.0:9090"

service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [prometheus]
```

Configure Piri to send metrics to your collector:

```toml
[[telemetry.metrics]]
endpoint = "http://localhost:4317"
insecure = true
publish_interval = "30s"
```

### Jaeger

For distributed tracing, configure a Jaeger backend with OTLP support:

```toml
[[telemetry.traces]]
endpoint = "http://jaeger:4317"
insecure = true
```

### Grafana

Connect your Prometheus datasource and create dashboards using the metrics above. Key metrics to monitor:

- **System health**: `system_cpu_utilization`, `system_memory_used_bytes`, `piri_datadir_free_bytes`
- **Job queue health**: `active_jobs`, `failed_jobs`, `job_duration`
- **API performance**: `http.server.request.duration` (p95, p99)

## Configuration

See [Configuration > telemetry](../configuration/telemetry.md) for collector setup options.

## Analytics

Piri can optionally send anonymized analytics to Storacha to help improve the software. See [Operations > Telemetry](../operations/telemetry.md) for details and opt-out instructions.
