# status

Check node status and health.

This command requires a running Piri server. Pass the same `--config` file you used with `piri serve` so the command knows where to connect:

```bash
piri --config=config.toml status
```

This command provides a quick overview of your node's current state, including whether it's healthy, in a challenge window, and safe to update. For more detailed information about your proof set (challenge timing, on-chain state, fee information), use [`piri client pdp proofset state`](../client/pdp/proofset/state.md).

## Usage

```
piri status [command] [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |

## Subcommands

### [upgrade-check](upgrade-check.md)

Check if it's safe to upgrade the node.

## Example

```bash
piri status
```

```
Node Status
===========
Healthy:           YES
Currently Proving: NO
In Challenge:      NO
Has Proven:        YES
In Fault State:    NO
Safe to Update:    YES
Next Challenge:    2025-01-28 14:30:00 UTC
```
