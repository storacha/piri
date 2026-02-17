# repo

Storage directory configuration.

| Key | Default           | Env | Dynamic |
|-----|-------------------|-----|---------|
| `repo.data_dir` | `$HOME/.storacha/` | `PIRI_REPO_DATA_DIR` | No |
| `repo.temp_dir` | -                 | `PIRI_REPO_TEMP_DIR` | No |

## Fields

### `data_dir`

Directory for persistent data (databases, blobs, claims).

### `temp_dir`

Directory for temporary files during processing.

## TOML

```toml
[repo]
data_dir = "/data/piri"
temp_dir = "/tmp/piri"
```

