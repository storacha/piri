# server

HTTP server configuration.

| Key                 | Default                | Env                      | Dynamic |
|---------------------|------------------------|--------------------------|---------|
| `server.port`       | `3000`                 | `PIRI_SERVER_PORT`       | No      |
| `server.host`       | `0.0.0.0`              | `PIRI_SERVER_HOST`       | No      |
| `server.public_url` | `http://{host}:{port}` | `PIRI_SERVER_PUBLIC_URL` | No      |

## Fields

### `port`

HTTP listening port (1-65535).

### `host`

HTTP listening host/interface.

### `public_url`

Externally accessible URL. Defaults to `http://{host}:{port}` if not set.

## TOML

```toml
[server]
port = 3000
host = "0.0.0.0"
public_url = "https://piri.example.com"
```
