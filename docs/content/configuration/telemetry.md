# telemetry

Observability configuration.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `telemetry.disable_storacha_analytics` | `false` | `PIRI_TELEMETRY_DISABLE_STORACHA_ANALYTICS` | No |

## Fields

### `disable_storacha_analytics`

Disable sending analytics to Storacha.

## [telemetry.metrics]

Configure metrics collectors (array).

```toml
[telemetry.metrics]
endpoint = "https://otel.example.com:4317"
insecure = false
publish_interval = "30s"

[telemetry.metrics.headers]
Authorization = "Bearer ..."
```

## [telemetry.traces]

Configure trace collectors (array).

```toml
[telemetry.traces]
endpoint = "https://otel.example.com:4317"
insecure = false

[telemetry.traces.headers]
Authorization = "Bearer ..."
```
