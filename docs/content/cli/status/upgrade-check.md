# upgrade-check

Check if it's safe to upgrade the node.

## Usage

```
piri status upgrade-check
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Safe to upgrade |
| `1` | Not safe to upgrade |
| `2` | Unable to determine status |

## Example

```bash
piri status upgrade-check && piri update
```

```
Safe to upgrade
```

This command is designed for use in scripts and automation. It returns exit codes to indicate whether an upgrade can safely proceed without interrupting proof generation.
