# update

Check for and apply updates to Piri.

## Usage

```
piri update [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--check` | Check for updates without applying them |
| `--force` | Skip safety checks and force update |
| `--version <version>` | Update to a specific version (e.g., v1.2.3) |

## Example

```bash
piri update
```

```
Current version: v0.2.1
Latest version:  v0.2.2
Downloading v0.2.2...
Update complete. Restart the service to apply changes.
```

This command downloads and installs the latest version but does not restart the service. Use `piri status upgrade-check` first to verify it's safe to update.
