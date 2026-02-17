# database

Database backend configuration.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `repo.database.type` | `sqlite` | `PIRI_REPO_DATABASE_TYPE` | No |
| `repo.database.postgres.url` | - | `PIRI_REPO_DATABASE_POSTGRES_URL` | No |
| `repo.database.postgres.max_open_conns` | `5` | `PIRI_REPO_DATABASE_POSTGRES_MAX_OPEN_CONNS` | No |
| `repo.database.postgres.max_idle_conns` | `5` | `PIRI_REPO_DATABASE_POSTGRES_MAX_IDLE_CONNS` | No |
| `repo.database.postgres.conn_max_lifetime` | `30m` | `PIRI_REPO_DATABASE_POSTGRES_CONN_MAX_LIFETIME` | No |

## Fields

### `type`

Database backend: `sqlite` (default) or `postgres`.

> **Important**: Database type cannot be changed after initial setup. Data is not migrated between backends. See [Database Concepts](../../concepts/database.md) for details.

### `postgres.url`

PostgreSQL connection string. Required when `type` is `postgres`.

Format: `postgres://user:password@host:port/dbname?sslmode=disable`

### `postgres.max_open_conns`

Maximum number of open connections to the database.

### `postgres.max_idle_conns`

Maximum number of idle connections in the pool.

### `postgres.conn_max_lifetime`

Maximum lifetime for a connection. Accepts Go duration strings (e.g., `30m`, `1h`).

## TOML

SQLite (default - no configuration needed):

```toml
[repo]
data_dir = "/data/piri"
```

PostgreSQL:

```toml
[repo.database]
type = "postgres"

[repo.database.postgres]
url = "postgres://piri:secret@localhost:5432/piri?sslmode=disable"
max_open_conns = 10
max_idle_conns = 5
conn_max_lifetime = "30m"
```
