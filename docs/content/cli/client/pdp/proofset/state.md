# state

Get the current state of a proof set.

## Usage

```
piri client pdp proofset state [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--proof-set <id>` | The proof set ID to query |
| `--json` | Output in JSON format |

## Example

```bash
piri client pdp proofset state --proof-set 123
```

```
Proof Set State
===============
Proof Set ID:        123
Proving Period:      2880 epochs
Challenge Window:    60 epochs

Owners
------
Owner:               0x1234...abcd

Status
------
Initialized:         true
Current Epoch:       4052160

Challenge State
---------------
In Challenge Window: false
Has Proven:          true
Next Challenge:      2025-01-28 14:30:00 UTC (estimated)

On-Chain State
--------------
Root Count:          42
Total Data Size:     1.2 TiB
```
